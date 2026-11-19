package http

import (
	"net/http"
)

func configIndexRoutes() {

	//前端已通过 go:embed 嵌入二进制（src/http/static.go），
	//直接使用内嵌静态资源服务。
	fs := staticHandler()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !AuthUserIp(r.RemoteAddr) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, 401)
			return
		}
		fs.ServeHTTP(w, r)
	})

}
