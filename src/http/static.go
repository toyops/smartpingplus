package http

import (
	"embed"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/smartping/smartping/src/g"
)

//静态前端产物（由 web/ 项目 vite build 输出到本目录，经 go:embed 嵌入二进制）。
//部署时为单文件自包含，无需携带 html/ 目录。
//
//注意：修改前端后需重新构建（cd web && npm run build），再重新编译 Go 二进制。
//
//例外：/alert.mp3 报警声音不嵌入（体积与可自定义考虑），
//运行时从可执行文件旁的 alert.mp3 磁盘文件读取；不存在时返回 404。

//go:embed static
var staticFS embed.FS

//staticHandler 返回内嵌前端资源的 HTTP 处理器（/alert.mp3 走磁盘回退）。
func staticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	embedFS := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//报警声音不嵌入：从磁盘提供，便于用户替换自定义声音
		if strings.HasSuffix(r.URL.Path, "/alert.mp3") {
			mp3 := filepath.Join(g.Root, "alert.mp3")
			if g.IsExist(mp3) {
				http.ServeFile(w, r, mp3)
				return
			}
			http.NotFound(w, r)
			return
		}
		embedFS.ServeHTTP(w, r)
	})
}
