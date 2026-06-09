param(
    [string]$Question = "支付系统最近是否有异常？请你自己查询可用的诊断工具，给出证据、影响范围和建议。",
    [string]$BaseUrl = "http://127.0.0.1:6872/api",
    [string]$ConversationId = ("utf8-chat-" + (Get-Date -Format "yyyyMMddHHmmss"))
)

$body = @{
    id = $ConversationId
    userId = "local"
    conversationId = $ConversationId
    question = $Question
} | ConvertTo-Json -Depth 8

Invoke-RestMethod `
    -Method Post `
    -Uri "$BaseUrl/chat" `
    -ContentType "application/json; charset=utf-8" `
    -Body ([System.Text.Encoding]::UTF8.GetBytes($body))
