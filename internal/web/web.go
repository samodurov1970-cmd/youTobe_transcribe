package web

import (
	"html/template"
	"log/slog"
	"net/http"
	"strconv"

	"youTobe_transcribe/internal/storage"
)

var trashTmpl = template.Must(template.New("trash").Parse(`
<!doctype html>
<html lang="ru">
<head>
<meta charset="utf-8">
<title>Trash</title>
<style>
  body { font-family: system-ui, sans-serif; margin: 2rem; background:#fafafa; }
  table { border-collapse: collapse; width: 100%; background:#fff; }
  th, td { border: 1px solid #ddd; padding: .4rem .6rem; text-align: left; vertical-align: top; }
  th { background: #f0f0f0; }
  .pending { color: #b36b00; font-weight: 600; }
  .ok { color: #2e7d32; }
  select { padding: .2rem; }
  form { margin: 0; }
</style>
</head>
<body>
<h1>Trash ({{len .Items}} записей)</h1>
<table>
  <thead>
    <tr>
      <th>ID</th><th>video_id</th><th>channel</th><th>reason</th>
      <th>created_at</th><th>action</th><th>действие</th>
    </tr>
  </thead>
  <tbody>
  {{range .Items}}
    <tr>
      <td>{{.ID}}</td>
      <td>{{.VideoID}}</td>
      <td>{{with .ChannelName}}{{.}}{{else}}—{{end}}</td>
      <td>{{with .Reason}}{{.}}{{else}}—{{end}}</td>
      <td>{{with .CreatedAt}}{{.Format "2006-01-02 15:04"}}{{else}}—{{end}}</td>
      <td>
        {{if .Action.Valid}}
          <span class="pending">{{.Action.String}} (в очереди)</span>
        {{else}}
          <span class="ok">—</span>
        {{end}}
      </td>
      <td>
        <form method="post" action="/trash/action">
          <input type="hidden" name="id" value="{{.ID}}">
          <select name="action" onchange="this.form.submit()">
            <option value="">— выбрать —</option>
            <option value="delete" {{if and .Action.Valid (eq .Action.String "delete")}}selected{{end}}>delete</option>
            <option value="reprocess" {{if and .Action.Valid (eq .Action.String "reprocess")}}selected{{end}}>reprocess</option>
          </select>
        </form>
      </td>
    </tr>
  {{else}}
    <tr><td colspan="7">Пусто</td></tr>
  {{end}}
  </tbody>
</table>
<p><a href="/trash">Обновить</a></p>
</body>
</html>
`))

func TrashPage(log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := storage.ListTrash(200)
		if err != nil {
			log.Error("trash.list_failed", "error", err.Error())
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := trashTmpl.Execute(w, map[string]any{"Items": items}); err != nil {
			log.Error("trash.render_failed", "error", err.Error())
		}
	}
}

func TrashSetAction(log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		action := r.FormValue("action")
		if action == "" {
			http.Redirect(w, r, "/trash", http.StatusSeeOther)
			return
		}
		if err := storage.SetTrashAction(id, action); err != nil {
			log.Error("trash.set_action_failed", "id", id, "action", action, "error", err.Error())
			http.Error(w, "set action failed", http.StatusInternalServerError)
			return
		}
		log.Info("trash.action_set", "id", id, "action", action)
		http.Redirect(w, r, "/trash", http.StatusSeeOther)
	}
}