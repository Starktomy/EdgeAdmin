(function () {
	"use strict"
	if (!window.axios || !window.fetch) { return }
	axios.interceptors.request.use(function (config) {
		var method = (config.method || "get").toUpperCase()
		var url = new URL(config.url, window.location.href)
		if (/^(GET|HEAD|OPTIONS)$/.test(method) || url.origin !== window.location.origin) {
			return config
		}
		return fetch("/csrf/token", {
			credentials: "same-origin", cache: "no-store",
			headers: {"X-Requested-With": "XMLHttpRequest"}
		}).then(function (response) {
			if (!response.ok) { throw new Error("无法验证表单，请刷新后重试") }
			return response.json()
		}).then(function (response) {
			if (response.code !== 200 || !response.data || !response.data.token) {
				throw new Error("无法验证表单，请刷新后重试")
			}
			config.headers = config.headers || {}
			config.headers["X-CSRF-Token"] = response.data.token
			return config
		})
	})
})()
