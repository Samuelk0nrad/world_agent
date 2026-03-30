package schema

type Error struct {
	Code    string `json:"error_code"`
	Message string `json:"message"`
}
