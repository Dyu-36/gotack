#!/bin/bash


GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

morph_clone_slide() {
    local deck=$1
    local from_slide=$2
    local to_slide=$3

    echo -e "${BLUE}📋 Cloning slide $from_slide → $to_slide...${NC}"
    officecli add "$deck" '/' --from "/slide[$from_slide]"

    echo -e "${BLUE}⚡ Setting morph transition...${NC}"
    officecli set "$deck" "/slide[$to_slide]" --prop transition=morph

    echo -e "${BLUE}📊 Listing shapes for ghosting reference:${NC}"
    officecli get "$deck" "/slide[$to_slide]" --depth 1

    echo -e "${BLUE}🔍 Verifying transition...${NC}"
    local trans=$(officecli get "$deck" "/slide[$to_slide]" --json 2>/dev/null | grep '"transition": "morph"')
    if [ -z "$trans" ]; then
        echo -e "${RED}❌ ERROR: Transition not set on slide $to_slide!${NC}"
        echo -e "${RED}   This slide will not have morph animation.${NC}"
        exit 1
    else
        echo -e "${GREEN}✅ Transition verified on slide $to_slide${NC}"
    fi

    echo ""
}

morph_ghost_content() {
    local deck=$1
    local slide=$2
    shift 2
    local shapes=("$@")

    if [ ${#shapes[@]} -eq 0 ]; then
        echo -e "${YELLOW}⚠️  No shapes to ghost${NC}"
        return 0
    fi

    echo -e "${BLUE}👻 Ghosting ${#shapes[@]} content shape(s) on slide $slide...${NC}"

    for shape_idx in "${shapes[@]}"; do
        officecli set "$deck" "/slide[$slide]/shape[$shape_idx]" --prop x=36cm 2>/dev/null
        if [ $? -eq 0 ]; then
            echo -e "${GREEN}  ✓ Ghosted shape[$shape_idx]${NC}"
        else
            echo -e "${RED}  ✗ Failed to ghost shape[$shape_idx]${NC}"
        fi
    done

    echo -e "${GREEN}✅ Ghosting complete${NC}"
    echo ""
}

morph_verify_slide() {
    local deck=$1
    local slide=$2

    echo -e "${BLUE}🔍 Verifying slide $slide...${NC}"

    local has_error=0

    local trans=$(officecli get "$deck" "/slide[$slide]" --json 2>/dev/null | grep '"transition": "morph"')
    if [ -z "$trans" ]; then
        echo -e "${RED}  ❌ Missing transition=morph${NC}"
        echo -e "${RED}     Without this, slide will not animate!${NC}"
        has_error=1
    else
        echo -e "${GREEN}  ✅ Transition OK${NC}"
    fi

    local prev_slide=$((slide - 1))
    if [ $prev_slide -ge 1 ]; then
        local shapes_json=$(officecli get "$deck" "/slide[$slide]" --json 2>/dev/null)

        local unghosted_check
        unghosted_check=$(printf '%s' "$shapes_json" | python3 -c "
import sys, json
try:
    data = json.load(sys.stdin)

    def check_children(children, prev_slide):
        unghosted = []
        for child in children:
            name = child.get('format', {}).get('name', '')
            x = child.get('format', {}).get('x', '')
            path = child.get('path', '')

            if f'#s{prev_slide}-' in name:
                if x != '36cm':
                    unghosted.append(f\"{path}: name={name}, x={x}\")

            if 'children' in child:
                unghosted.extend(check_children(child['children'], prev_slide))

        return unghosted

    if 'children' in data.get('data', {}):
        unghosted = check_children(data['data']['children'], $prev_slide)

        if unghosted:
            for item in unghosted:
                print(item)
            sys.exit(1)

    sys.exit(0)
except Exception as e:
    print(f'[helper] parse error: {e}', file=sys.stderr)
    sys.exit(2)
")
        local python_exit=$?

        if [ $python_exit -eq 1 ] && [ -n "$unghosted_check" ]; then
            echo -e "${YELLOW}  ⚠️  Warning: Found unghosted content from slide $prev_slide:${NC}"
            echo "$unghosted_check" | sed 's/^/     /'
            echo -e "${YELLOW}     These shapes should be ghosted to x=36cm${NC}"
            has_error=1
        else
            echo -e "${GREEN}  ✅ No unghosted content detected${NC}"
        fi
    fi

    if [ $prev_slide -ge 1 ]; then
        local prev_json=$(officecli get "$deck" "/slide[$prev_slide]" --json 2>/dev/null)
        local curr_json="$shapes_json"

        local duplicates
        duplicates=$(python3 -c "
import sys, json

try:
    prev_data = json.loads('''$prev_json''')
    curr_data = json.loads('''$curr_json''')

    def extract_textboxes(data, slide_num):
        boxes = []
        def walk(children):
            for child in children:
                if child.get('type') == 'textbox':
                    name = child.get('format', {}).get('name', '')
                    text = child.get('text', '').strip()
                    x = child.get('format', {}).get('x', '')
                    y = child.get('format', {}).get('y', '')
                    path = child.get('path', '')

                    if not text or len(text) < 6:
                        continue

                    clean_name = name.replace('!!', '') if name else ''

                    scene_keywords = ['ring', 'dot', 'line', 'circle', 'rect', 'slash',
                                     'accent', 'actor', 'star', 'triangle', 'diamond']
                    is_scene = any(kw in clean_name.lower() for kw in scene_keywords)

                    has_slide_pattern = any(f's{i}-' in clean_name for i in range(1, 20))

                    if has_slide_pattern or not is_scene:
                        boxes.append({
                            'path': path,
                            'name': name,
                            'text': text[:50],
                            'x': x,
                            'y': y
                        })

                if 'children' in child:
                    walk(child['children'])

        if 'children' in data.get('data', {}):
            walk(data['data']['children'])
        return boxes

    prev_boxes = extract_textboxes(prev_data, $prev_slide)
    curr_boxes = extract_textboxes(curr_data, $slide)

    duplicates = []
    for curr in curr_boxes:
        for prev in prev_boxes:
            if (curr['text'] == prev['text'] and
                curr['x'] == prev['x'] and
                curr['y'] == prev['y']):
                if curr['x'] != '36cm':
                    duplicates.append(f\"{curr['path']}: text='{curr['text']}...', pos=({curr['x']},{curr['y']})\")
                break

    if duplicates:
        for dup in duplicates:
            print(dup)
        sys.exit(1)

    sys.exit(0)
except Exception as e:
    print(f'[helper] parse error: {e}', file=sys.stderr)
    sys.exit(2)
")

        local dup_exit=$?

        if [ $dup_exit -eq 1 ] && [ -n "$duplicates" ]; then
            echo -e "${YELLOW}  ⚠️  Warning: Found duplicate content from slide $prev_slide (same text at same position):${NC}"
            echo "$duplicates" | sed 's/^/     /'
            echo -e "${YELLOW}     This might indicate:${NC}"
            echo -e "${YELLOW}     1. Content shapes missing '#sN-' prefix (can't detect for ghosting)${NC}"
            echo -e "${YELLOW}     2. Forgot to ghost previous slide's content${NC}"
            echo -e "${YELLOW}     3. Forgot to add new content for this slide${NC}"
            has_error=1
        fi
    fi

    if [ $has_error -eq 0 ]; then
        echo -e "${GREEN}✅ Slide $slide verification passed${NC}"
    else
        echo -e "${RED}❌ Slide $slide has issues - see above${NC}"
        return 1
    fi

    echo ""
}

morph_final_check() {
    local deck=$1

    echo -e "${BLUE}🎯 Final deck verification...${NC}"
    echo ""

    local total_slides=$(officecli view "$deck" outline 2>/dev/null | head -1 | grep -oE '[0-9]+' | head -1 || echo "0")

    if [ "$total_slides" -eq 0 ]; then
        echo -e "${RED}❌ No slides found in deck${NC}"
        return 1
    fi

    echo "Total slides: $total_slides"
    echo ""

    local error_count=0

    for ((i=2; i<=total_slides; i++)); do
        if ! morph_verify_slide "$deck" "$i"; then
            ((error_count++))
        fi
    done

    echo ""
    echo "========================================="
    if [ $error_count -eq 0 ]; then
        echo -e "${GREEN}✅ All slides verified successfully!${NC}"
        echo -e "${GREEN}   Your morph animations should work correctly.${NC}"
        return 0
    else
        echo -e "${RED}❌ Found issues in $error_count slide(s)${NC}"
        echo -e "${RED}   Please fix the issues above before delivering.${NC}"
        return 1
    fi
}

if [ "${BASH_SOURCE[0]}" == "${0}" ]; then
    echo "Morph PPT Helper Functions"
    echo ""
    echo "Usage: source morph-helpers.sh"
    echo ""
    echo "Available functions:"
    echo "  morph_clone_slide <deck> <from> <to>      - Clone slide and set transition"
    echo "  morph_ghost_content <deck> <slide> <idx...> - Ghost multiple shapes"
    echo "  morph_verify_slide <deck> <slide>         - Verify slide setup"
    echo "  morph_final_check <deck>                  - Verify entire deck"
    echo ""
    echo "Example:"
    echo "  source morph-helpers.sh"
    echo "  morph_clone_slide deck.pptx 1 2"
    echo "  morph_ghost_content deck.pptx 2 7 8"
    echo "  morph_verify_slide deck.pptx 2"
fi
