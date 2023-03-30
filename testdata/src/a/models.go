package a

type Todo struct {
	ID   string `json:"id"`
	Done bool   `json:"done"`
}

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
