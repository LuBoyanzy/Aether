import { defineConfig } from "vite"
import path from "path"
import tailwindcss from "@tailwindcss/vite"
import react from "@vitejs/plugin-react"
import { lingui } from "@lingui/vite-plugin"

export default defineConfig({
	base: "./",
	server: { host: "0.0.0.0", port: 19091 },
	plugins: [
		react({
			babel: {
				plugins: ["macros"],
			},
		}),
		lingui(),
		tailwindcss(),
	],
	esbuild: {
		legalComments: "external",
	},
	resolve: {
		alias: {
			"@": path.resolve(__dirname, "./src"),
		},
	},
})
