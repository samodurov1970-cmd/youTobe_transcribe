$content = Get-Content internal/channels/channels.go -Raw -Encoding UTF8

# Change signarure: (int, error) -> (int, []string, error)
$content = $content -replace [regex]::Escape('(int, error) {'), '(int, []string, error) {', $content

Set-Content internal/channels/channels.go -Value $content -Encoding UTF8