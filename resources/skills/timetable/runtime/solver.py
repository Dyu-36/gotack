#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Bộ xếp thời khóa biểu có sẵn của skill timetable (CP-SAT, 2 pha).

Cách chạy:
    python -X utf8 runtime/solver.py <problem.json> <schedule.json> [options]

Tùy chọn CLI:
    --phase-a-only        Chỉ chạy Pha A (tìm nghiệm hợp lệ) rồi xuất kết quả ngay.
    --time-budget <sec>   Tổng ngân sách thời gian (giây), tự chia 70% pha A và 30% pha B.
    --diagnose            Chạy chẩn đoán nhóm yêu cầu mâu thuẫn khi vô nghiệm.
    --verbose             In chi tiết log solver và toàn bộ bảng kiểm PASS/FAIL.

problem.json mô tả: khung thời gian (frame), phân công (assignments),
yêu cầu bắt buộc (requirements), ưu tiên mềm (preferences).
Schema chi tiết: reference/problem-schema.md cùng skill.

Nếu cạnh problem.json có tệp constraints_extra.py, solver tự nạp và gọi
register(reg) để thêm yêu cầu đặc thù (xem problem-schema.md, mục API mở rộng).

Mã thoát: 0 thành công | 2 vô nghiệm/quá trần thời gian | 3 problem.json lỗi
          | 4 thiếu ortools | 5 lỗi plugin | 6 tự kiểm tra FAIL.
"""

import argparse 
import json 
import os 
import sys 
import threading 
import time 
import traceback 
from collections import defaultdict 

if hasattr (sys .stdout ,"reconfigure"):
    sys .stdout .reconfigure (encoding ="utf-8")
if hasattr (sys .stderr ,"reconfigure"):
    sys .stderr .reconfigure (encoding ="utf-8")

try :
    from ortools .sat .python import cp_model 
except ImportError :
    sys .stderr .write (
    "Lỗi: Thư viện 'ortools' chưa được cài đặt. Hãy chạy: pip install ortools\n"
    )
    sys .exit (4 )

EXIT_OK =0 
EXIT_INFEASIBLE =2 
EXIT_BAD_INPUT =3 
EXIT_NO_ORTOOLS =4 
EXIT_PLUGIN =5 
EXIT_SELFCHECK =6 

SESSION_ORDER ={"Sáng":0 ,"Chiều":1 ,"Tối":2 }

SLOT_SELECTOR_KEYS ={"days","sessions","periods","from_start","from_end","periods_from"}
LESSON_SELECTOR_KEYS ={"teachers","subjects","classes"}

REQ_COMMON_KEYS ={"id","name","type","selector"}
REQ_TYPE_KEYS ={
"pin":{"slots","count"},
"forbid_slots":{"slot_selector","slots"},
"allow_slots":{"slot_selector","slots"},
"per_day_limit":{"per","max","min","exactly"},
"spread_days":{"slot_selector","min_days","exactly_days"},
"same_day_adjacent":set (),
"shared_days":{"pairs","min_days"},
"pair_days_disjoint":{"selector_b"},
"no_k_consecutive":{"per","k"},
"min_total_in_slots":{"slot_selector","slots","min"},
"class_slot_used":{"classes","slot","selector"},
"resource":{"resource","per_class_count","capacity","slot_selector","exclude_adjacent"},
}
TOP_LEVEL_KEYS ={"frame","assignments","requirements","preferences","solver","compact"}
SOLVER_KEYS ={"phase_a_seconds","phase_b_seconds","workers","random_seed"}

DAY_LABEL ={2 :"Thứ 2",3 :"Thứ 3",4 :"Thứ 4",5 :"Thứ 5",6 :"Thứ 6",7 :"Thứ 7",8 :"Chủ nhật"}


def die (msg ,code ):
    if msg :
        sys .stderr .write (msg .rstrip ()+"\n")
    sys .exit (code )


def slot_label (slot ):
    day ,sess ,period =slot 
    return f"{DAY_LABEL .get (day ,day )}-{sess }-tiết {period }"






class Frame :
    """Danh sách slot (day, session, period) suy ra từ frame.days."""

    def __init__ (self ,days ):
        self .days =days 
        self .slots =[]
        self .day_sessions ={}
        for d in days :
            self .day_sessions [d ["day"]]=[s ["name"]for s in d ["sessions"]]
            for sess in d ["sessions"]:
                for p in range (1 ,sess ["periods"]+1 ):
                    self .slots .append ((d ["day"],sess ["name"],p ))
        self .slots_by_day =defaultdict (list )
        self .slots_by_day_session =defaultdict (list )
        self ._session_len ={}
        for s in self .slots :
            self .slots_by_day [s [0 ]].append (s )
            self .slots_by_day_session [(s [0 ],s [1 ])].append (s )
            self ._session_len [(s [0 ],s [1 ])]=max (
            self ._session_len .get ((s [0 ],s [1 ]),0 ),s [2 ]
            )
        self .slot_set =set (self .slots )

    def session_len (self ,slot ):
        return self ._session_len [(slot [0 ],slot [1 ])]

    def sort_key (self ,slot ):
        day ,sess ,period =slot 
        return (day ,SESSION_ORDER .get (sess ,5 ),period )

    def neighbors (self ,slot ):
        day ,sess ,period =slot 
        out =[]
        for dp in (-1 ,1 ):
            nb =(day ,sess ,period +dp )
            if nb in self .slot_set :
                out .append (nb )
        return out 

    def adjacent (self ,s1 ,s2 ):
        return s1 [1 ]==s2 [1 ]and abs (s1 [2 ]-s2 [2 ])==1 


def match_slot_selector (frame ,sel ,explicit_slots =None ):
    """Danh sách slot khớp bộ chọn. explicit_slots ưu tiên hơn sel.

    sel có thể là một dict (các điều kiện giao nhau) hoặc list các dict
    (hợp kết quả của từng dict — dùng cho cửa sổ kiểu "Chiều hoặc Sáng từ tiết 3").
    """
    if explicit_slots is not None :
        out =[]
        for s in explicit_slots :
            key =(int (s ["day"]),str (s ["session"]),int (s ["period"]))
            if key not in frame .slot_set :
                raise ValueError (f"slot không tồn tại trong khung: {s }")
            out .append (key )
        return out 
    if not sel :
        return list (frame .slots )
    if isinstance (sel ,list ):
        seen =set ()
        merged =[]
        for sub in sel :
            for s in match_slot_selector (frame ,sub ):
                if s not in seen :
                    seen .add (s )
                    merged .append (s )
        return merged 
    days =sel .get ("days")
    sessions =sel .get ("sessions")
    periods =sel .get ("periods")
    from_start =sel .get ("from_start")
    from_end =sel .get ("from_end")
    periods_from =sel .get ("periods_from")
    out =[]
    for s in frame .slots :
        day ,sess ,period =s 
        if days is not None and day not in days :
            continue 
        if sessions is not None and sess not in sessions :
            continue 
        if periods is not None and period not in periods :
            continue 
        if from_start is not None and period >from_start :
            continue 
        if periods_from is not None and period <periods_from :
            continue 
        n =frame .session_len (s )
        if from_end is not None and period <=n -from_end :
            continue 
        out .append (s )
    return out 


def normalize_selector (sel ,allowed ,what ):
    if sel is None :
        return {}
    if not isinstance (sel ,dict ):
        raise ValueError (f"{what } phải là đối tượng")
    unknown =set (sel .keys ())-allowed 
    if unknown :
        raise ValueError (f"{what } chứa khóa lạ: {sorted (unknown )}")
    out =dict (sel )
    return out 


def _is_int (v ):
    return isinstance (v ,int )and not isinstance (v ,bool )






def validate_problem (problem ):
    errors =[]

    def err (msg ):
        errors .append (msg )

    if not isinstance (problem ,dict ):
        return ["problem.json phải là một đối tượng JSON"]
    unknown_top =set (problem .keys ())-TOP_LEVEL_KEYS 
    if unknown_top :
        err (f"khóa lạ ở mức gốc: {sorted (unknown_top )}")

    frame =problem .get ("frame")
    if not isinstance (frame ,dict )or not isinstance (frame .get ("days"),list )or not frame ["days"]:
        err ("thiếu frame.days (khung thời gian)")
        frame =None 
    else :
        for i ,d in enumerate (frame ["days"]):
            if not isinstance (d ,dict )or not _is_int (d .get ("day"))or not 2 <=d ["day"]<=8 :
                err (f"frame.days[{i }].day phải là số từ 2 đến 8")
            if not isinstance (d .get ("sessions"),list )or not d ["sessions"]:
                err (f"frame.days[{i }].sessions không được trống")
                continue 
            for j ,sess in enumerate (d ["sessions"]):
                if not isinstance (sess ,dict )or not isinstance (sess .get ("name"),str ):
                    err (f"frame.days[{i }].sessions[{j }].name phải là chuỗi")
                if not _is_int (sess .get ("periods"))or sess ["periods"]<1 :
                    err (f"frame.days[{i }].sessions[{j }].periods phải là số nguyên ≥ 1")

    rows =problem .get ("assignments")
    if not isinstance (rows ,list )or not rows :
        err ("thiếu assignments (danh sách phân công)")
        rows =[]
    seen_ids =set ()
    for i ,r in enumerate (rows ):
        if not isinstance (r ,dict ):
            err (f"assignments[{i }] phải là đối tượng")
            continue 
        for field in ("teacher","class","subject"):
            if not isinstance (r .get (field ),str )or not r [field ].strip ():
                err (f"assignments[{i }] thiếu '{field }' hợp lệ")
        if not _is_int (r .get ("periods"))or r ["periods"]<1 :
            err (f"assignments[{i }].periods phải là số nguyên ≥ 1")
        rid =r .get ("id")
        if rid is not None :
            if rid in seen_ids :
                err (f"assignments[{i }]: id trùng '{rid }'")
            seen_ids .add (rid )

    reqs =problem .get ("requirements")or []
    if not isinstance (reqs ,list ):
        err ("requirements phải là danh sách")
        reqs =[]
    for i ,req in enumerate (reqs ):
        if not isinstance (req ,dict ):
            err (f"requirements[{i }] phải là đối tượng")
            continue 
        t =req .get ("type")
        if t not in REQ_TYPE_KEYS :
            err (
            f"requirements[{i }] ({req .get ('id','?')}): loại không hỗ trợ '{t }'. "
            f"Loại hỗ trợ: {sorted (REQ_TYPE_KEYS )}. "
            "Nếu yêu cầu không diễn đạt được bằng các loại này, hãy viết hàm trong "
            "constraints_extra.py (xem reference/problem-schema.md)."
            )
            continue 
        unknown =set (req .keys ())-REQ_COMMON_KEYS -REQ_TYPE_KEYS [t ]
        if unknown :
            err (f"requirements[{i }] ({req .get ('id','?')}): khóa lạ {sorted (unknown )}")
        sel =req .get ("selector")
        if sel is not None and not isinstance (sel ,dict ):
            err (f"requirements[{i }].selector phải là đối tượng")
        elif sel is not None and set (sel .keys ())-LESSON_SELECTOR_KEYS :
            err (
            f"requirements[{i }].selector chứa khóa lạ: "
            f"{sorted (set (sel .keys ())-LESSON_SELECTOR_KEYS )}"
            )
        for ss_field in ("slot_selector",):
            ss =req .get (ss_field )
            if ss is None :
                continue 
            subs =ss if isinstance (ss ,list )else [ss ]
            if not isinstance (ss ,(dict ,list ))or not all (
            isinstance (x ,dict )for x in subs 
            ):
                err (f"requirements[{i }].slot_selector phải là đối tượng hoặc danh sách đối tượng")
                continue 
            for x in subs :
                unknown =set (x .keys ())-SLOT_SELECTOR_KEYS 
                if unknown :
                    err (
                    f"requirements[{i }].slot_selector chứa khóa lạ: {sorted (unknown )}"
                    )
        if t =="pin"and not req .get ("slots"):
            err (f"requirements[{i }] ({req .get ('id','?')}): pin cần 'slots'")
        if t =="no_k_consecutive"and not _is_int (req .get ("k")):
            err (f"requirements[{i }] ({req .get ('id','?')}): cần 'k' nguyên")
        if t =="class_slot_used"and not req .get ("classes"):
            err (f"requirements[{i }] ({req .get ('id','?')}): cần 'classes'")
        if t =="resource"and not req .get ("resource"):
            err (f"requirements[{i }] ({req .get ('id','?')}): resource cần 'resource' (tên tài nguyên)")

    prefs =problem .get ("preferences")or []
    if not isinstance (prefs ,list ):
        err ("preferences phải là danh sách")
        prefs =[]
    for i ,pref in enumerate (prefs ):
        if not isinstance (pref ,dict ):
            err (f"preferences[{i }] phải là đối tượng")
            continue 
        allowed ={"id","name","selector","slot_selector","weight","avoid"}
        unknown =set (pref .keys ())-allowed 
        if unknown :
            err (f"preferences[{i }]: khóa lạ {sorted (unknown )}")

    sol =problem .get ("solver")
    if sol is not None :
        if not isinstance (sol ,dict ):
            err ("solver phải là đối tượng")
        else :
            unknown =set (sol .keys ())-SOLVER_KEYS 
            if unknown :
                err (f"solver chứa khóa lạ: {sorted (unknown )}")

    return errors 






class PluginBuildAPI :
    """API cho hàm build() của plugin constraints_extra.py."""

    def __init__ (self ,builder ,lit ):
        self ._b =builder 
        self ._lit =lit 

    @property 
    def model (self ):
        return self ._b .model 

    def rows (self ,selector =None ):
        return self ._b .rows_for (selector or {})

    def slot_keys (self ,slot_selector =None ):
        return match_slot_selector (self ._b .frame ,slot_selector )

    def x (self ,row_id ,slot ):
        return self ._b .x [(row_id ,slot )]

    def occ (self ,selector ,slots ):
        rows =self ._b .rows_for (selector or {})
        return sum (self ._b .x [(r ["id"],s )]for r in rows for s in slots )

    def add_hard (self ,expr ):
        self ._b .model .add (expr )

    def add_under (self ,expr ):
        self ._b ._add (expr ,self ._lit )

    def add_penalty (self ,expr ,weight ,note =""):
        self ._b .penalties .append ((expr ,weight ,note ))

    def new_bool (self ,name ):
        return self ._b .model .new_bool_var (name )


class PluginCheckAPI :
    """API cho hàm check() của plugin: chấm trên lịch đã xếp."""

    def __init__ (self ,builder ,placed ):
        self ._b =builder 
        self .placed =placed 

    def rows (self ,selector =None ):
        return self ._b .rows_for (selector or {})

    def slot_keys (self ,slot_selector =None ):
        return match_slot_selector (self ._b .frame ,slot_selector )

    def placed_of (self ,selector =None ,slots =None ):
        sel =selector or {}
        rows ={r ["id"]for r in self ._b .rows_for (sel )}
        slot_set =set (slots )if slots is not None else None 
        return [
        p 
        for p in self .placed 
        if p ["row_id"]in rows and (slot_set is None or p ["slot"]in slot_set )
        ]


class Builder :
    def __init__ (self ,problem ):
        self .problem =problem 
        self .frame =Frame (problem ["frame"]["days"])
        self .rows =[]
        for i ,a in enumerate (problem ["assignments"]):
            row =dict (a )
            row .setdefault ("id",f"a{i }")
            self .rows .append (row )
        self .row_by_id ={r ["id"]:r for r in self .rows }
        self .model =cp_model .CpModel ()
        self .x ={}
        for r in self .rows :
            for s in self .frame .slots :
                self .x [(r ["id"],s )]=self .model .new_bool_var (
                f"x_{r ['id']}_{s [0 ]}_{s [1 ]}_{s [2 ]}"
                )
        self .assumption_names ={}
        self .assume_mode =False 
        self .penalties =[]
        self .requirements =[]
        self .plugin_requirements =[]
        self .resource_labels ={}
        self ._occ_cache ={}

    def _add (self ,expr ,lit ):
        if lit is not None :
            return self .model .add (expr ).only_enforce_if (lit )
        return self .model .add (expr )

    def _add_at_most_one (self ,literals ,lit ):



        if lit is not None :
            return self .model .add (sum (literals )<=1 ).only_enforce_if (lit )
        return self .model .add_at_most_one (literals )

    def _add_exactly_one (self ,literals ,lit ):
        if lit is not None :
            return self .model .add (sum (literals )==1 ).only_enforce_if (lit )
        return self .model .add_exactly_one (literals )

    def _add_bool_or (self ,literals ,lit ):
        if lit is not None :
            return self .model .add_bool_or (literals ).only_enforce_if (lit )
        return self .model .add_bool_or (literals )



    def rows_for (self ,sel ):
        if not sel :
            return list (self .rows )
        out =[]
        for r in self .rows :
            if sel .get ("teachers")and r ["teacher"]not in sel ["teachers"]:
                continue 
            if sel .get ("subjects")and r ["subject"]not in sel ["subjects"]:
                continue 
            if sel .get ("classes")and r ["class"]not in sel ["classes"]:
                continue 
            out .append (r )
        return out 

    def occ (self ,rows ,slot ):
        key =(tuple (sorted (r ["id"]for r in rows )),slot )
        if key not in self ._occ_cache :
            self ._occ_cache [key ]=sum (self .x [(r ["id"],slot )]for r in rows )
        return self ._occ_cache [key ]

    def class_ids (self ,rows ):
        return sorted ({r ["class"]for r in rows })

    def teacher_ids (self ,rows ):
        return sorted ({r ["teacher"]for r in rows })

    def new_lit (self ,rid ):
        """Công tắc của một yêu cầu.

        Chế độ hard (mặc định, assume_mode=False): trả về None để ghim ràng buộc trực tiếp,
        không tạo biến bool thừa và không dùng only_enforce_if (tận dụng tối đa presolve).
        Chế độ assume (chẩn đoán vô nghiệm): literal làm assumption để lấy nhóm yêu cầu mâu thuẫn.
        """
        if self .assume_mode :
            lit =self .model .new_bool_var (f"req_{rid }")
            self .assumption_names [lit .index ]=rid 
            self .model .add_assumption (lit )
            return lit 
        return None 



    def build_core (self ):
        m =self .model 
        for r in self .rows :
            p =int (r ["periods"])
            vars_r =[self .x [(r ["id"],s )]for s in self .frame .slots ]
            if p ==1 :
                m .add_exactly_one (vars_r )
            else :
                m .add (sum (vars_r )==p )

        for c in self .class_ids (self .rows ):
            crows =[r for r in self .rows if r ["class"]==c ]
            for s in self .frame .slots :
                m .add_at_most_one ([self .x [(r ["id"],s )]for r in crows ])

        for t in self .teacher_ids (self .rows ):
            trows =[r for r in self .rows if r ["teacher"]==t ]
            for s in self .frame .slots :
                m .add_at_most_one ([self .x [(r ["id"],s )]for r in trows ])



        if self .problem .get ("compact",False ):
            for c in self .class_ids (self .rows ):
                crows =[r for r in self .rows if r ["class"]==c ]
                for key ,slots in self .frame .slots_by_day_session .items ():
                    ordered =sorted (slots ,key =lambda s :s [2 ])
                    for prev ,cur in zip (ordered ,ordered [1 :]):
                        m .add (self .occ (crows ,cur )<=self .occ (crows ,prev ))



    def add_requirement (self ,req ,index ):
        t =req ["type"]
        rid =req .get ("id")or f"Y{index }"
        name =req .get ("name")or rid 
        sel =normalize_selector (req .get ("selector"),LESSON_SELECTOR_KEYS ,f"yêu cầu {rid }.selector")
        lit =self .new_lit (rid )
        m =self .model 
        rows =self .rows_for (sel )

        def slot_set ():
            if req .get ("slots")is not None :
                return match_slot_selector (self .frame ,None ,req ["slots"])
            return match_slot_selector (self .frame ,req .get ("slot_selector"))

        if t =="pin":
            count =int (req .get ("count",1 ))
            slots =slot_set ()
            for r in rows :
                vars_pin =[self .x [(r ["id"],s )]for s in slots ]
                if count ==1 and len (vars_pin )==1 :
                    self ._add (vars_pin [0 ]==1 ,lit )
                elif count ==1 and len (vars_pin )>1 :
                    self ._add_exactly_one (vars_pin ,lit )
                else :
                    self ._add (sum (vars_pin )==count ,lit )

        elif t in ("forbid_slots","allow_slots"):
            if t =="allow_slots":
                allowed =set (slot_set ())
                slots =[s for s in self .frame .slots if s not in allowed ]
            else :
                slots =slot_set ()
            for r in rows :
                for s in slots :
                    self ._add (self .x [(r ["id"],s )]==0 ,lit )

        elif t =="per_day_limit":
            per =req .get ("per","class")
            if per not in ("class","teacher"):
                raise ValueError (f"yêu cầu {rid }: per phải là 'class' hoặc 'teacher'")
            get_key =(lambda r :r ["class"])if per =="class"else (lambda r :r ["teacher"])
            groups =sorted ({get_key (r )for r in rows })
            for g in groups :
                grows =[r for r in rows if get_key (r )==g ]
                for day in self .frame .day_sessions :
                    day_slots =self .frame .slots_by_day [day ]
                    day_vars =[self .x [(r ["id"],s )]for r in grows for s in day_slots ]
                    if req .get ("max")is not None :
                        max_val =int (req ["max"])
                        if max_val ==1 :
                            self ._add_at_most_one (day_vars ,lit )
                        else :
                            self ._add (sum (day_vars )<=max_val ,lit )
                    if req .get ("min")is not None :
                        self ._add (sum (day_vars )>=int (req ["min"]),lit )
                    if req .get ("exactly")is not None :
                        eq_val =int (req ["exactly"])
                        if eq_val ==1 :
                            self ._add_exactly_one (day_vars ,lit )
                        else :
                            self ._add (sum (day_vars )==eq_val ,lit )

        elif t =="spread_days":
            counting =set (slot_set ())
            for c in self .class_ids (rows ):
                crows =[r for r in rows if r ["class"]==c ]
                day_used =[]
                for day in self .frame .day_sessions :
                    day_slots =[s for s in self .frame .slots_by_day [day ]if s in counting ]
                    if not day_slots :
                        continue 
                    terms =[self .x [(r ["id"],s )]for r in crows for s in day_slots ]
                    if not terms :
                        continue 
                    du =self .model .new_bool_var (f"du_{rid }_{c }_{day }")
                    m .add_max_equality (du ,terms )
                    day_used .append (du )
                total =sum (day_used )
                if req .get ("min_days")is not None :
                    self ._add (total >=int (req ["min_days"]),lit )
                if req .get ("exactly_days")is not None :
                    self ._add (total ==int (req ["exactly_days"]),lit )

        elif t =="same_day_adjacent":
            for c in self .class_ids (rows ):
                crows =[r for r in rows if r ["class"]==c ]
                for day ,day_slots in self .frame .slots_by_day .items ():
                    n_slots =len (day_slots )
                    for i in range (n_slots ):
                        for j in range (i +1 ,n_slots ):
                            s1 ,s2 =day_slots [i ],day_slots [j ]
                            if self .frame .adjacent (s1 ,s2 ):
                                continue 
                            vars_pair =[self .x [(r ["id"],s1 )]for r in crows ]+[
                            self .x [(r ["id"],s2 )]for r in crows 
                            ]
                            self ._add_at_most_one (vars_pair ,lit )

        elif t =="shared_days":
            min_days =int (req .get ("min_days",1 ))
            for c1 ,c2 in req .get ("pairs")or []:
                if c1 ==c2 :
                    continue 
                shared =[]
                for day in self .frame .day_sessions :
                    day_slots =self .frame .slots_by_day [day ]
                    t1 =[self .x [(r ["id"],s )]for r in rows if r ["class"]==c1 for s in day_slots ]
                    t2 =[self .x [(r ["id"],s )]for r in rows if r ["class"]==c2 for s in day_slots ]
                    if not t1 or not t2 :
                        continue 
                    u1 =self .model .new_bool_var (f"sh_{rid }_{c1 }_{day }")
                    u2 =self .model .new_bool_var (f"sh_{rid }_{c2 }_{day }")
                    m .add_max_equality (u1 ,t1 )
                    m .add_max_equality (u2 ,t2 )
                    both =self .model .new_bool_var (f"both_{rid }_{c1 }_{c2 }_{day }")
                    m .add_bool_or ([u1 .Not (),u2 .Not (),both ])
                    m .add_implication (both ,u1 )
                    m .add_implication (both ,u2 )
                    shared .append (both )
                self ._add (sum (shared )>=min_days ,lit )

        elif t =="pair_days_disjoint":
            sel_b =normalize_selector (
            req .get ("selector_b"),LESSON_SELECTOR_KEYS ,f"yêu cầu {rid }.selector_b"
            )
            rows_b =self .rows_for (sel_b )
            for c in sorted (set (self .class_ids (rows ))&set (self .class_ids (rows_b ))):
                crows_a =[r for r in rows if r ["class"]==c ]
                crows_b =[r for r in rows_b if r ["class"]==c ]
                for day ,day_slots in self .frame .slots_by_day .items ():
                    pa =self .model .new_bool_var (f"pda_{rid }_{c }_{day }")
                    pb =self .model .new_bool_var (f"pdb_{rid }_{c }_{day }")
                    sum_a =sum (self .x [(r ["id"],s )]for r in crows_a for s in day_slots )
                    sum_b =sum (self .x [(r ["id"],s )]for r in crows_b for s in day_slots )
                    m .add (sum_a <=1 ).only_enforce_if (pa .Not ())
                    m .add (sum_a >=2 ).only_enforce_if (pa )
                    m .add (sum_b <=1 ).only_enforce_if (pb .Not ())
                    m .add (sum_b >=2 ).only_enforce_if (pb )
                    self ._add_bool_or ([pa .Not (),pb .Not ()],lit )

        elif t =="no_k_consecutive":
            per =req .get ("per","class")
            k =int (req ["k"])
            if per not in ("class","teacher"):
                raise ValueError (f"yêu cầu {rid }: per phải là 'class' hoặc 'teacher'")
            get_key =(lambda r :r ["class"])if per =="class"else (lambda r :r ["teacher"])
            groups =sorted ({get_key (r )for r in rows })
            for g in groups :
                grows =[r for r in rows if get_key (r )==g ]
                for key ,slots in self .frame .slots_by_day_session .items ():
                    ordered =sorted (slots ,key =lambda s :s [2 ])
                    for start in range (0 ,len (ordered )-k +1 ):
                        window =ordered [start :start +k ]
                        window_vars =[self .x [(r ["id"],s )]for r in grows for s in window ]
                        self ._add (sum (window_vars )<=k -1 ,lit )

        elif t =="min_total_in_slots":
            slots =slot_set ()
            total =sum (self .x [(r ["id"],s )]for r in rows for s in slots )
            self ._add (total >=int (req ["min"]),lit )

        elif t =="class_slot_used":
            slot =req ["slot"]
            key =(int (slot ["day"]),str (slot ["session"]),int (slot ["period"]))
            if key not in self .frame .slot_set :
                raise ValueError (f"yêu cầu {rid }: slot không tồn tại: {slot }")
            sub_sel =req .get ("selector")
            if sub_sel is not None :
                sub_sel =normalize_selector (
                sub_sel ,LESSON_SELECTOR_KEYS ,f"yêu cầu {rid }.selector"
                )
            allowed_ids ={r ["id"]for r in self .rows_for (sub_sel or {})}
            for c in req ["classes"]:
                crows =[r for r in self .rows if r ["class"]==c and r ["id"]in allowed_ids ]
                if not crows :
                    raise ValueError (
                    f"yêu cầu {rid }: lớp {c } không có phân công nào khớp selector"
                    )
                cvars =[self .x [(r ["id"],key )]for r in crows ]
                if len (cvars )==1 :
                    self ._add (cvars [0 ]==1 ,lit )
                else :
                    self ._add_exactly_one (cvars ,lit )

        elif t =="resource":
            res_name =req ["resource"]
            allowed =set (slot_set ())
            if not allowed :
                raise ValueError (f"yêu cầu {rid }: cửa sổ tài nguyên rỗng")
            cap =int (req .get ("capacity",1 ))
            u ={}
            per_slot =defaultdict (list )
            for r in rows :
                allowed_slots =[s for s in self .frame .slots if s in allowed ]
                for s in allowed_slots :
                    uv =self .model .new_bool_var (
                    f"res_{res_name }_{r ['id']}_{s [0 ]}_{s [1 ]}_{s [2 ]}"
                    )
                    u [(r ["id"],s )]=uv 
                    self ._add (uv <=self .x [(r ["id"],s )],lit )
                    per_slot [s ].append (uv )


            for c in self .class_ids (rows ):
                crows =[r for r in rows if r ["class"]==c ]
                if req .get ("per_class_count")is not None :
                    uid =int (req ["per_class_count"])
                else :
                    uid =sum (int (r ["periods"])for r in crows )
                cvars =[u [(r ["id"],s )]for r in crows for s in self .frame .slots if (r ["id"],s )in u ]
                if not cvars :
                    raise ValueError (
                    f"yêu cầu {rid }: lớp {c } không có slot nào trong cửa sổ tài nguyên"
                    )
                if uid ==1 :
                    self ._add_exactly_one (cvars ,lit )
                else :
                    self ._add (sum (cvars )==uid ,lit )
            for s ,uvs in per_slot .items ():
                if cap ==1 :
                    self ._add_at_most_one (uvs ,lit )
                else :
                    self ._add (sum (uvs )<=cap ,lit )
            excl =req .get ("exclude_adjacent")
            if excl :
                sel_b =normalize_selector (
                excl ,LESSON_SELECTOR_KEYS ,f"yêu cầu {rid }.exclude_adjacent"
                )
                rows_b =self .rows_for (sel_b )
                for (row_id ,s ),uv in u .items ():
                    row =self .row_by_id [row_id ]
                    brows =[r for r in rows_b if r ["class"]==row ["class"]]
                    for nb in self .frame .neighbors (s ):
                        if brows :
                            nb_vars =[uv ]+[self .x [(r ["id"],nb )]for r in brows ]
                            self ._add_at_most_one (nb_vars ,lit )
            self .resource_labels [res_name ]=u 

        self .requirements .append ((rid ,name ,req ))



    def build_objective (self ):
        m =self .model 
        terms =[]
        by_cs =defaultdict (list )
        for r in self .rows :
            by_cs [(r ["class"],r ["subject"])].append (r )
        for (c ,sub ),rlist in by_cs .items ():
            for day ,day_slots in self .frame .slots_by_day .items ():
                total =sum (self .x [(r ["id"],s )]for r in rlist for s in day_slots )
                over =self .model .new_int_var (0 ,10 ,f"ov_{c }_{sub }_{day }")
                m .add (over >=total -1 )
                terms .append ((over ,3 ))
        by_teacher =defaultdict (list )
        for r in self .rows :
            by_teacher [r ["teacher"]].append (r )
        for t ,trows in by_teacher .items ():
            for key ,slots in self .frame .slots_by_day_session .items ():
                ordered =sorted (slots ,key =lambda s :s [2 ])
                n =len (ordered )
                if n <3 :
                    continue 
                busy ={s :self .occ (trows ,s )for s in ordered }
                before =[None ]*n 
                after =[None ]*n 
                for i in range (1 ,n ):
                    b =self .model .new_bool_var (f"bf_{t }_{key }_{i }")
                    m .add_max_equality (b ,[busy [ordered [j ]]for j in range (i )])
                    before [i ]=b 
                for i in range (n -1 ):
                    a =self .model .new_bool_var (f"af_{t }_{key }_{i }")
                    m .add_max_equality (a ,[busy [ordered [j ]]for j in range (i +1 ,n )])
                    after [i ]=a 
                for i in range (n ):
                    if before [i ]is None or after [i ]is None :
                        continue 
                    gap =self .model .new_bool_var (f"gap_{t }_{key }_{i }")
                    m .add (gap <=1 -busy [ordered [i ]])
                    m .add (gap <=before [i ])
                    m .add (gap <=after [i ])
                    m .add (gap >=before [i ]+after [i ]-busy [ordered [i ]]-1 )
                    terms .append ((gap ,2 ))
        for i ,pref in enumerate (self .problem .get ("preferences")or []):
            sel =normalize_selector (
            pref .get ("selector"),LESSON_SELECTOR_KEYS ,f"ưu tiên {pref .get ('id',i )}"
            )
            weight =int (pref .get ("weight",1 ))
            avoid =bool (pref .get ("avoid",False ))
            chosen =set (match_slot_selector (self .frame ,pref .get ("slot_selector")))
            rows =self .rows_for (sel )
            if avoid :
                expr_terms =[self .x [(r ["id"],s )]for r in rows for s in chosen ]
            else :
                expr_terms =[
                self .x [(r ["id"],s )]
                for r in rows 
                for s in self .frame .slots 
                if s not in chosen 
                ]
            if expr_terms :
                terms .append ((sum (expr_terms ),weight ))
        for expr ,weight ,_note in self .penalties :
            terms .append ((expr ,weight ))
        if not terms :
            return None 
        return sum (int (w )*e for e ,w in terms )






class EarlyStoppingSolutionCallback (cp_model .CpSolverSolutionCallback ):
    """Dừng Pha B nếu không có cải thiện thêm trong khoảng thời gian quy định."""

    def __init__ (self ,stop_after_no_improve_seconds =5.0 ):
        super ().__init__ ()
        self ._stop_seconds =stop_after_no_improve_seconds 
        self ._best_objective =None 
        self ._timer =None 
        self ._done =False 
        self ._lock =threading .Lock ()




    def _arm_timer (self ):
        with self ._lock :
            if self ._done :
                return 
            if self ._timer is not None :
                self ._timer .cancel ()
            self ._timer =threading .Timer (self ._stop_seconds ,self ._on_timeout )
            self ._timer .daemon =True 
            self ._timer .start ()

    def _on_timeout (self ):


        with self ._lock :
            if self ._done :
                return 
            self .stop_search ()

    def on_solution_callback (self ):
        current_obj =self .objective_value 
        if self ._best_objective is None or current_obj <self ._best_objective -1e-4 :
            self ._best_objective =current_obj 
            self ._arm_timer ()

    def close (self ):
        with self ._lock :
            self ._done =True 
            if self ._timer is not None :
                self ._timer .cancel ()
                self ._timer =None 






def check_core (builder ,placed ):
    fails =[]
    counts =defaultdict (int )
    for p in placed :
        counts [p ["row_id"]]+=1 
    for r in builder .rows :
        if counts [r ["id"]]!=int (r ["periods"]):
            fails .append (
            f"LÕI-1: số tiết {r ['teacher']} - {r ['subject']} - {r ['class']} là "
            f"{counts [r ['id']]} thay vì {r ['periods']}"
            )
    cls_slot =defaultdict (list )
    tch_slot =defaultdict (list )
    for p in placed :
        cls_slot [(p ["class"],p ["slot"])].append (p )
        tch_slot [(p ["teacher"],p ["slot"])].append (p )
    for key ,lst in cls_slot .items ():
        if len (lst )>1 :
            fails .append (f"LÕI-2: lớp {key [0 ]} bị trùng tại {slot_label (key [1 ])}")
    for key ,lst in tch_slot .items ():
        if len (lst )>1 :
            fails .append (
            f"LÕI-2: giáo viên {key [0 ]} bị trùng tại {slot_label (key [1 ])}"
            )
    if builder .problem .get ("compact",False ):
        by_cs =defaultdict (set )
        for p in placed :
            by_cs [(p ["class"],p ["slot"][0 ],p ["slot"][1 ])].add (p ["slot"][2 ])
        for c in builder .class_ids (builder .rows ):
            for (day ,sess ),slots in builder .frame .slots_by_day_session .items ():
                used =by_cs .get ((c ,day ,sess ),set ())
                if not used :
                    continue 
                if used !=set (range (1 ,max (used )+1 )):
                    fails .append (
                    f"LÕI-3: lớp {c } {DAY_LABEL .get (day ,day )}-{sess } bị trống tiết giữa buổi"
                    )
    return fails 


def check_requirement (builder ,rid ,req ,placed ):
    """Trả về danh sách lỗi (rỗng = PASS) chấm trên dữ liệu lịch."""
    t =req ["type"]
    sel =req .get ("selector")or {}
    rows =builder .rows_for (sel )
    row_ids ={r ["id"]for r in rows }
    mine =[p for p in placed if p ["row_id"]in row_ids ]
    fails =[]

    def req_slots ():
        if req .get ("slots")is not None :
            return set (match_slot_selector (builder .frame ,None ,req ["slots"]))
        return set (match_slot_selector (builder .frame ,req .get ("slot_selector")))

    if t =="pin":
        count =int (req .get ("count",1 ))
        slots =req_slots ()
        for r in rows :
            n =sum (1 for p in placed if p ["row_id"]==r ["id"]and p ["slot"]in slots )
            if n !=count :
                fails .append (f"{rid }: {r ['subject']}-{r ['class']} ghim {n }/{count } tiết đúng chỗ")

    elif t in ("forbid_slots","allow_slots"):
        if t =="forbid_slots":
            bad_slots =req_slots ()
        else :
            allowed =req_slots ()
            bad_slots =set (builder .frame .slots )-allowed 
        for p in mine :
            if p ["slot"]in bad_slots :
                fails .append (
                f"{rid }: {p ['subject']}-{p ['class']} rơi vào slot không được phép "
                f"({slot_label (p ['slot'])})"
                )

    elif t =="per_day_limit":
        per =req .get ("per","class")
        get_key =(lambda p :p ["class"])if per =="class"else (lambda p :p ["teacher"])
        counts =defaultdict (int )
        for p in mine :
            counts [(get_key (p ),p ["slot"][0 ])]+=1 
        for (g ,day ),n in sorted (counts .items ()):
            if req .get ("max")is not None and n >int (req ["max"]):
                fails .append (f"{rid }: {g } có {n } tiết/ngày {DAY_LABEL .get (day ,day )} (tối đa {req ['max']})")
            if req .get ("min")is not None and n <int (req ["min"]):
                fails .append (f"{rid }: {g } chỉ {n } tiết/ngày {DAY_LABEL .get (day ,day )} (tối thiểu {req ['min']})")
            if req .get ("exactly")is not None and n !=int (req ["exactly"]):
                fails .append (f"{rid }: {g } có {n } tiết/ngày {DAY_LABEL .get (day ,day )} (yêu cầu đúng {req ['exactly']})")

    elif t =="spread_days":
        counting =req_slots ()
        for c in builder .class_ids (rows ):
            days ={p ["slot"][0 ]for p in mine if p ["class"]==c and p ["slot"]in counting }
            if req .get ("min_days")is not None and len (days )<int (req ["min_days"]):
                fails .append (f"{rid }: {c } chỉ trải {len (days )} ngày (tối thiểu {req ['min_days']})")
            if req .get ("exactly_days")is not None and len (days )!=int (req ["exactly_days"]):
                fails .append (f"{rid }: {c } trải {len (days )} ngày (yêu cầu đúng {req ['exactly_days']})")

    elif t =="same_day_adjacent":
        by_cd =defaultdict (list )
        for p in mine :
            by_cd [(p ["class"],p ["slot"][0 ])].append (p ["slot"])
        for (c ,day ),slots in by_cd .items ():
            for i in range (len (slots )):
                for j in range (i +1 ,len (slots )):
                    if not builder .frame .adjacent (slots [i ],slots [j ]):
                        fails .append (
                        f"{rid }: {c } {DAY_LABEL .get (day ,day )} có 2 tiết không liền kề cùng buổi"
                        )

    elif t =="shared_days":
        min_days =int (req .get ("min_days",1 ))
        for c1 ,c2 in req .get ("pairs")or []:
            d1 ={p ["slot"][0 ]for p in mine if p ["class"]==c1 }
            d2 ={p ["slot"][0 ]for p in mine if p ["class"]==c2 }
            shared =d1 &d2 
            if len (shared )<min_days :
                fails .append (f"{rid }: {c1 }/{c2 } chỉ trùng {len (shared )} ngày (tối thiểu {min_days })")

    elif t =="pair_days_disjoint":
        rows_b =builder .rows_for (req .get ("selector_b")or {})
        ids_b ={r ["id"]for r in rows_b }
        cnt_a =defaultdict (int )
        cnt_b =defaultdict (int )
        for p in mine :
            cnt_a [(p ["class"],p ["slot"][0 ])]+=1 
        for p in placed :
            if p ["row_id"]in ids_b :
                cnt_b [(p ["class"],p ["slot"][0 ])]+=1 
        for key in sorted (set (cnt_a )&set (cnt_b )):
            if cnt_a [key ]>=2 and cnt_b [key ]>=2 :
                fails .append (
                f"{rid }: {key [0 ]} ngày {DAY_LABEL .get (key [1 ],key [1 ])} có tiết đôi của cả hai nhóm"
                )

    elif t =="no_k_consecutive":
        per =req .get ("per","class")
        k =int (req ["k"])
        get_key =(lambda p :p ["class"])if per =="class"else (lambda p :p ["teacher"])
        by_group =defaultdict (set )
        for p in mine :
            by_group [(get_key (p ),p ["slot"][0 ],p ["slot"][1 ])].add (p ["slot"][2 ])
        for (g ,day ,sess ),periods in sorted (by_group .items ()):
            ordered =sorted (periods )
            run =1 
            for a ,b in zip (ordered ,ordered [1 :]):
                if b ==a +1 :
                    run +=1 
                    if run >=k :
                        fails .append (
                        f"{rid }: {g } dạy {k } tiết liền tại {DAY_LABEL .get (day ,day )}-{sess }"
                        )
                        run =1 
                else :
                    run =1 

    elif t =="min_total_in_slots":
        slots =req_slots ()
        n =sum (1 for p in mine if p ["slot"]in slots )
        if n <int (req ["min"]):
            fails .append (f"{rid }: chỉ có {n } tiết trong cửa sổ yêu cầu (tối thiểu {req ['min']})")

    elif t =="class_slot_used":
        slot =req ["slot"]
        key =(int (slot ["day"]),str (slot ["session"]),int (slot ["period"]))
        ids ={r ["id"]for r in builder .rows_for (req .get ("selector")or {})}
        for c in req ["classes"]:
            n =sum (
            1 
            for p in placed 
            if p ["class"]==c and p ["slot"]==key and p ["row_id"]in ids 
            )
            if n !=1 :
                fails .append (f"{rid }: lớp {c } có {n } tiết (yêu cầu 1) tại {slot_label (key )}")

    elif t =="resource":
        res_name =req ["resource"]
        allowed =req_slots ()
        mine_res =[p for p in mine if res_name in p ["labels"]]
        per_slot =defaultdict (int )
        for p in mine_res :
            per_slot [p ["slot"]]+=1 
            if p ["slot"]not in allowed :
                fails .append (f"{rid }: tiết thực hành {p ['class']} ngoài cửa sổ ({slot_label (p ['slot'])})")
        for s ,n in per_slot .items ():
            if n >int (req .get ("capacity",1 )):
                fails .append (f"{rid }: {n } lớp cùng dùng tài nguyên tại {slot_label (s )}")
        for c in builder .class_ids (rows ):
            if req .get ("per_class_count")is not None :
                want =int (req ["per_class_count"])
            else :
                want =sum (int (r ["periods"])for r in rows if r ["class"]==c )
            n =sum (1 for p in mine_res if p ["class"]==c )
            if n !=want :
                fails .append (f"{rid }: {c } có {n }/{want } tiết dùng tài nguyên {res_name }")
        excl =req .get ("exclude_adjacent")
        if excl :
            ids_b ={x ["id"]for x in builder .rows_for (excl )}
            for p in mine_res :
                for nb in builder .frame .neighbors (p ["slot"]):
                    for q in placed :
                        if q ["row_id"]in ids_b and q ["slot"]==nb and q ["class"]==p ["class"]:
                            fails .append (
                            f"{rid }: tiết thực hành {p ['class']} kề {q ['subject']} tại "
                            f"{slot_label (nb )}"
                            )

    return fails 






def load_plugin (builder ,problem_path ):
    """Nạp constraints_extra.py nằm cạnh problem.json nếu tồn tại."""
    plugin_path =os .path .join (os .path .dirname (os .path .abspath (problem_path )),"constraints_extra.py")
    if not os .path .isfile (plugin_path ):
        return 
    try :
        import importlib .util 

        spec =importlib .util .spec_from_file_location ("constraints_extra",plugin_path )
        mod =importlib .util .module_from_spec (spec )
        spec .loader .exec_module (mod )
    except Exception :
        sys .stderr .write (f"Lỗi khi nạp plugin {plugin_path }:\n{traceback .format_exc ()}\n")
        sys .exit (EXIT_PLUGIN )
    register =getattr (mod ,"register",None )
    if register is None :
        sys .stderr .write (
        f"Plugin {plugin_path } phải định nghĩa hàm register(reg). "
        "Xem reference/problem-schema.md mục API mở rộng.\n"
        )
        sys .exit (EXIT_PLUGIN )

    registry =[]

    def requirement (name ,build ,check =None ):
        registry .append ((name ,build ,check ))

    register (requirement =requirement )
    for name ,build ,check in registry :
        rid =f"PLUGIN:{name }"
        lit =builder .new_lit (rid )
        api =PluginBuildAPI (builder ,lit )
        try :
            build (api )
        except Exception :
            sys .stderr .write (
            f"Lỗi khi dựng ràng buộc plugin '{name }':\n{traceback .format_exc ()}\n"
            )
            sys .exit (EXIT_PLUGIN )
        builder .requirements .append ((rid ,name ,{"type":"__plugin__","name":name }))
        builder .plugin_requirements .append ((name ,check ))


def run_plugin_checks (builder ,placed ):
    results =[]
    for name ,check in builder .plugin_requirements :
        if check is None :
            results .append ((f"PLUGIN:{name }",name ,None ,"đã giải trong mô hình, không có hàm kiểm tra riêng"))
            continue 
        capi =PluginCheckAPI (builder ,placed )
        try :
            out =check (capi )
        except Exception :
            out =(False ,"hàm kiểm tra plugin lỗi: "+traceback .format_exc (limit =1 ).strip ())
        if isinstance (out ,tuple ):
            ok ,detail =out [0 ],(out [1 ]if len (out )>1 else "")
        else :
            ok ,detail =out ,""
        results .append ((f"PLUGIN:{name }",name ,bool (ok ),detail ))
    return results 






def read_solution (builder ,solver ):
    placed =[]
    for r in builder .rows :
        for s in builder .frame .slots :
            if solver .boolean_value (builder .x [(r ["id"],s )]):
                placed .append (
                {
                "row_id":r ["id"],
                "teacher":r ["teacher"],
                "class":r ["class"],
                "subject":r ["subject"],
                "slot":s ,
                "labels":[],
                }
                )
    for res_name ,u in builder .resource_labels .items ():
        for (row_id ,s ),var in u .items ():
            if solver .boolean_value (var ):
                for p in placed :
                    if p ["row_id"]==row_id and p ["slot"]==s :
                        p ["labels"].append (res_name )
    return placed 


def hint_from (builder ,solver ):
    for (row_id ,s ),var in builder .x .items ():
        builder .model .add_hint (var ,solver .boolean_value (var ))
    for res_name ,u in builder .resource_labels .items ():
        for key ,var in u .items ():
            builder .model .add_hint (var ,solver .boolean_value (var ))


def write_schedule (builder ,placed ,out_path ):
    data ={
    "classes":builder .class_ids (builder .rows ),
    "days":builder .problem ["frame"]["days"],
    "assignments":[
    {
    "day":p ["slot"][0 ],
    "session":p ["slot"][1 ],
    "period":p ["slot"][2 ],
    "class":p ["class"],
    "subject":p ["subject"],
    "teacher":p ["teacher"],
    **({"labels":p ["labels"]}if p ["labels"]else {}),
    }
    for p in sorted (
    placed ,key =lambda p :(builder .frame .sort_key (p ["slot"]),p ["class"])
    )
    ],
    }
    os .makedirs (os .path .dirname (os .path .abspath (out_path ))or ".",exist_ok =True )
    with open (out_path ,"w",encoding ="utf-8")as f :
        json .dump (data ,f ,ensure_ascii =False ,indent =2 )


class _ArgParser (argparse .ArgumentParser ):
    """Giữ nguyên quy ước mã thoát của skill.

    argparse mặc định thoát với mã 2 khi sai tham số, nhưng mã 2 ở đây có nghĩa là
    "vô nghiệm" — agent sẽ đi nới ràng buộc vô ích. Sai cú pháp phải là lỗi đầu vào (3).
    """

    def error (self ,message ):
        self .print_usage (sys .stderr )
        die (f"Tham số dòng lệnh không hợp lệ: {message }",EXIT_BAD_INPUT )


def main ():
    parser =_ArgParser (
    description ="Bộ xếp thời khóa biểu trường học (CP-SAT 2 pha)."
    )
    parser .add_argument ("problem",help ="Đường dẫn file problem.json")
    parser .add_argument ("output",help ="Đường dẫn file schedule.json")
    parser .add_argument (
    "--phase-a-only",
    action ="store_true",
    help ="Chỉ chạy Pha A (tìm nghiệm hợp lệ) rồi xuất kết quả ngay.",
    )
    parser .add_argument (
    "--time-budget",
    type =float ,
    default =None ,
    help ="Tổng ngân sách thời gian (giây), tự chia 70%% pha A và 30%% pha B.",
    )
    parser .add_argument (
    "--diagnose",
    action ="store_true",
    help ="Chạy chẩn đoán nhóm yêu cầu mâu thuẫn khi bài toán vô nghiệm.",
    )
    parser .add_argument (
    "--verbose",
    action ="store_true",
    help ="In chi tiết tiến trình tìm kiếm và toàn bộ bảng kiểm PASS/FAIL.",
    )

    args =parser .parse_args ()
    problem_path ,out_path =args .problem ,args .output 

    try :
        with open (problem_path ,encoding ="utf-8")as f :
            problem =json .load (f )
    except (OSError ,json .JSONDecodeError )as e :
        die (f"Không đọc được problem.json: {e }",EXIT_BAD_INPUT )

    errors =validate_problem (problem )
    if errors :
        sys .stderr .write ("problem.json không hợp lệ:\n")
        for e in errors :
            sys .stderr .write (f"  - {e }\n")
        sys .exit (EXIT_BAD_INPUT )

    cfg =problem .get ("solver")or {}
    if args .time_budget is not None and args .time_budget >0 :


        if args .phase_a_only :
            phase_a_seconds =float (args .time_budget )
            phase_b_seconds =0.0 
        else :
            phase_a_seconds =float (args .time_budget )*0.7 
            phase_b_seconds =float (args .time_budget )*0.3 
    else :

        want_a =float (cfg .get ("phase_a_seconds",20.0 ))
        want_b =float (cfg .get ("phase_b_seconds",15.0 ))
        phase_a_seconds =min (want_a ,60.0 )
        phase_b_seconds =min (want_b ,15.0 )


        if want_a >phase_a_seconds :
            print (
            f"CẢNH BÁO: solver.phase_a_seconds={want_a :.0f}s vượt trần cứng 60s, "
            f"đã hạ xuống {phase_a_seconds :.0f}s. Muốn chạy dài hơn thì dùng "
            f"--time-budget (không bị áp trần)."
            )
        if want_b >phase_b_seconds and not args .phase_a_only :
            print (
            f"CẢNH BÁO: solver.phase_b_seconds={want_b :.0f}s vượt trần cứng 15s, "
            f"đã hạ xuống {phase_b_seconds :.0f}s."
            )
    phase_a_seconds =max (phase_a_seconds ,1.0 )
    workers =int (cfg .get ("workers",min (os .cpu_count ()or 4 ,8 )))
    seed =int (cfg .get ("random_seed",7 ))

    def build (assume_mode ):
        b =Builder (problem )
        b .assume_mode =assume_mode 
        b .build_core ()
        for i ,req in enumerate (problem .get ("requirements")or []):
            try :
                b .add_requirement (req ,i )
            except ValueError as e :
                die (f"Lỗi yêu cầu: {e }",EXIT_BAD_INPUT )
        load_plugin (b ,problem_path )
        return b 

    builder =build (assume_mode =False )


    solver_a =cp_model .CpSolver ()
    solver_a .parameters .max_time_in_seconds =phase_a_seconds 
    solver_a .parameters .num_search_workers =workers 
    solver_a .parameters .random_seed =seed 
    solver_a .parameters .stop_after_first_solution =True 
    if args .verbose :
        solver_a .parameters .log_search_progress =True 

    t0 =time .time ()
    status_a =solver_a .solve (builder .model )
    secs_a =time .time ()-t0 

    if status_a ==cp_model .INFEASIBLE :
        print ("KẾT QUẢ: CHƯA XẾP ĐƯỢC (VÔ NGHIỆM)")
        if args .diagnose :

            diag =build (assume_mode =True )
            solver_d =cp_model .CpSolver ()
            diag_time =min (phase_a_seconds ,20.0 )
            solver_d .parameters .max_time_in_seconds =diag_time 
            solver_d .parameters .num_search_workers =workers 
            solver_d .parameters .random_seed =seed 
            if args .verbose :
                solver_d .parameters .log_search_progress =True 
            status_d =solver_d .solve (diag .model )
            names =[]
            if status_d ==cp_model .INFEASIBLE :
                conflicts =solver_d .sufficient_assumptions_for_infeasibility ()
                names =[
                diag .assumption_names [i ]
                for i in conflicts 
                if i in diag .assumption_names 
                ]
            if names :
                name_set =set (names )
                pretty =[n for n ,_nm ,_r in diag .requirements if n in name_set ]
                print ("CÁC YÊU CẦU MÂU THUẪN NHAU (cần nới bớt một trong các yêu cầu):")
                for n in pretty or sorted (name_set ):
                    print (f"  - {n }")
            elif status_d ==cp_model .INFEASIBLE :
                print (
                "Bản thân dữ liệu phân công mâu thuẫn với khung thời gian "
                "(thiếu slot, vượt tải lớp/giáo viên). Hãy rà lại tổng số tiết "
                "so với số slot của khung."
                )
            else :
                print (
                "Bài toán xác định là vô nghiệm nhưng chưa chỉ ra được nhóm yêu cầu "
                f"mâu thuẫn trong {diag_time :.0f}s chẩn đoán. "
                "Hãy nới bớt các yêu cầu ràng buộc (phòng học, lịch bận, tiết đôi...) "
                "hoặc kiểm tra dữ liệu phân công / số slot khả dụng."
                )
        else :
            print (
            "Bài toán không tìm được phương án thỏa mãn toàn bộ ràng buộc.\n"
            "Hãy nới bớt yêu cầu ràng buộc (lịch bận GV, phòng chức năng, phân bố tiết) "
            "hoặc kiểm tra dữ liệu phân công / số slot khả dụng.\n"
            "(Chạy lại với cờ `--diagnose` để phân tích chi tiết nhóm yêu cầu mâu thuẫn)."
            )
        die ("",EXIT_INFEASIBLE )

    if status_a not in (cp_model .OPTIMAL ,cp_model .FEASIBLE ):
        print ("KẾT QUẢ: CHƯA TÌM ĐƯỢC LỊCH HỢP LỆ TRONG THỜI GIAN CHO PHÉP")
        print (
        f"Pha tìm nghiệm chạy hết {secs_a :.1f}s mà chưa ra lịch. "
        "Hãy nới bớt yêu cầu ràng buộc hoặc kiểm tra dữ liệu phân công / số slot khả dụng."
        )
        die ("",EXIT_INFEASIBLE )


    placed =read_solution (builder ,solver_a )

    write_schedule (builder ,placed ,out_path )


    secs_b =0.0 
    improved =False 
    if not args .phase_a_only :
        objective =builder .build_objective ()
        if objective is not None :
            hint_from (builder ,solver_a )
            builder .model .minimize (objective )
            solver_b =cp_model .CpSolver ()
            solver_b .parameters .max_time_in_seconds =phase_b_seconds 
            solver_b .parameters .num_search_workers =workers 
            solver_b .parameters .random_seed =seed 
            solver_b .parameters .relative_gap_limit =0.05 
            if args .verbose :
                solver_b .parameters .log_search_progress =True 

            cb =EarlyStoppingSolutionCallback (stop_after_no_improve_seconds =5.0 )
            t1 =time .time ()
            try :
                status_b =solver_b .solve (builder .model ,cb )
            finally :
                cb .close ()
            secs_b =time .time ()-t1 
            if status_b in (cp_model .OPTIMAL ,cp_model .FEASIBLE ):
                placed =read_solution (builder ,solver_b )
                improved =True 

                write_schedule (builder ,placed ,out_path )


    fails =check_core (builder ,placed )
    table =[]
    for rid ,name ,req in builder .requirements :
        if req .get ("type")=="__plugin__":
            continue 
        req_fails =check_requirement (builder ,rid ,req ,placed )
        ok =not req_fails 
        table .append ((rid ,name ,ok ,"; ".join (req_fails [:3 ])))
        fails .extend (req_fails )
    for rid ,name ,ok ,detail in run_plugin_checks (builder ,placed ):
        table .append ((rid ,name ,ok if ok is not None else None ,detail ))
        if ok is False :
            fails .append (f"{rid }: {detail }")

    total =len (placed )
    n_pass =sum (1 for _r ,_n ,ok ,_d in table if ok is True )
    n_na =sum (1 for _r ,_n ,ok ,_d in table if ok is None )
    n_fail =sum (1 for _r ,_n ,ok ,_d in table if ok is False )

    print ("KẾT QUẢ: THÀNH CÔNG"if not fails else "KẾT QUẢ: LỖI TỰ KIỂM TRA")
    print (
    f"- Phân công: {len (builder .teacher_ids (builder .rows ))} giáo viên, "
    f"{len (builder .class_ids (builder .rows ))} lớp, {total } tiết/tuần"
    )
    print (
    f"- Kiểm tra: {n_pass }/{n_pass +n_fail } yêu cầu PASS"
    +(f", {n_na } không có hàm kiểm tra riêng"if n_na else "")
    +(""if not fails else f" — CÒN {len (fails )} LỖI")
    )
    if args .phase_a_only :
        print (f"- Thời gian: pha tìm nghiệm {secs_a :.1f}s (chỉ chạy Pha A)")
    else :
        print (
        f"- Thời gian: pha tìm nghiệm {secs_a :.1f}s, pha tinh chỉnh {secs_b :.1f}s"
        +(" (đã dùng bản tinh chỉnh)"if improved else " (giữ bản pha 1)")
        )
    print (f"- File kết quả: {out_path }")


    non_pass =[row for row in table if row [2 ]is not True ]
    if args .verbose :
        print ("- Bảng kiểm từng yêu cầu:")
        for rid ,name ,ok ,detail in table :
            mark ="PASS"if ok is True else ("??? "if ok is None else "FAIL")
            print (f"    [{mark }] {rid } — {name }"+(f" | {detail }"if detail and ok is not True else ""))
    elif non_pass :
        print ("- Các yêu cầu chưa đạt / cần lưu ý:")
        for rid ,name ,ok ,detail in non_pass :
            mark ="??? "if ok is None else "FAIL"
            print (f"    [{mark }] {rid } — {name }"+(f" | {detail }"if detail else ""))

    if fails :
        die ("",EXIT_SELFCHECK )
    sys .exit (EXIT_OK )


if __name__ =="__main__":
    main ()
