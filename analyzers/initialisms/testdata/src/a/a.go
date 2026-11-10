package a

type URLPath struct{}

type urlPath struct{}

type UrlPath struct{} // want `Url should be URL in UrlPath`

func f(httpURL string) {}

func g(httpUrl string) { // want `Url should be URL in httpUrl`
	_ = httpUrl
}

func h(rapid int) { _ = rapid }

func i(appID, appid, APPID string) {}
