package services

func WsKey(branch string, isMain bool) string {
	if isMain {
		return "main:" + branch
	}
	return "ws:" + branch
}
