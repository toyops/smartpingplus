import { defineConfig } from "vite"
import path from "path"
import tailwindcss from "@tailwindcss/vite"
import react from "@vitejs/plugin-react-swc"

// Build output goes to ../src/http/static so the Go backend can embed it
// into the binary via go:embed (src/http/static.go). The binary is then
// self-contained: no html/ directory needed at deploy time.
export default defineConfig({
	base: "./",
	plugins: [react(), tailwindcss()],
	resolve: {
		alias: { "@": path.resolve(__dirname, "./src") },
	},
	build: {
		outDir: path.resolve(__dirname, "../src/http/static"),
		emptyOutDir: true,
	},
	server: {
		port: 5173,
		proxy: {
			// dev-mode proxy to a running SmartPing backend
			"/api": "http://127.0.0.1:8899",
		},
	},
})
