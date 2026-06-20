package wechattoken

// 以下符号仅供 internal/service/wechattoken/test 外部测试包访问。

func WechatTokenAPIBaseForTest() string {
	return wechatTokenAPIBase
}

func SetWechatTokenAPIBaseForTest(url string) {
	wechatTokenAPIBase = url
}
