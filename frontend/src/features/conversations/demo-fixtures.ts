import { ChatMessage, type Conversation } from './types.svelte'

export function demoConversations(): Conversation[] {
  const m1 = new ChatMessage('msg-1', 'user', Date.now() - 60000)
  m1.content = 'Xin chào, hãy phân tích cho tôi cấu trúc dự án và hiển thị ví dụ mã nguồn.'

  const mTool = new ChatMessage('msg-tool-1', 'assistant', Date.now() - 50000)
  mTool.kind = 'tool'
  mTool.toolName = 'list_dir'
  mTool.toolFinished = true
  mTool.content = 'Scanning workspace D:/gotack...'

  const m2 = new ChatMessage('msg-2', 'assistant', Date.now() - 40000)
  m2.content = `Chào bạn! Dưới đây là phân tích cấu trúc dự án và ví dụ cụ thể:

### 1. Cấu trúc tổng quan
Dự án được chia thành các phần chính:
- **Frontend**: Svelte 5 + Tailwind CSS v4
- **Backend (Desktop host)**: Go + Wails v2
- **Engine**: REST + SSE communication

### 2. Bảng so sánh các thành phần
| Thành phần | Vai trò | Công nghệ |
| :--- | :--- | :--- |
| UI Frame | Giao diện người dùng | Svelte 5, Tailwind CSS |
| Desktop Host | Cầu nối hệ thống | Wails v2 (Go) |
| Agent Engine | Xử lý mô hình AI & tools | REST / SSE |

### 3. Ví dụ mã cấu hình
\`\`\`typescript
export function setupClient() {
  console.log("Gotack Client initialized!");
  return { status: "ready", timestamp: Date.now() };
}
\`\`\`

Bạn cần tôi hỗ trợ kiểm tra hay tinh chỉnh thêm phần nào không?`

  const m3 = new ChatMessage('msg-3', 'user', Date.now() - 20000)
  m3.content = 'Hãy kiểm tra vị trí của khung chat và độ cân đối của các tin nhắn.'

  const m4 = new ChatMessage('msg-4', 'assistant', Date.now() - 5000)
  m4.content = 'Tôi đang hiển thị câu trả lời mẫu để bạn kiểm tra vị trí tin nhắn của người dùng và mô hình trên giao diện.'

  return [{
    id: 'session-demo-1',
    title: 'Hội thoại mẫu kiểm tra giao diện',
    updatedAt: Date.now(),
    pinned: false,
    status: 'idle',
    messages: [m1, mTool, m2, m3, m4],
  }]
}
