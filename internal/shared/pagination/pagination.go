package pagination

type Request struct {
	Page     int `form:"page" validate:"gte=1,lte=1000000"`
	PageSize int `form:"pageSize" validate:"gte=1,lte=100"`
}
type Response[T any] struct {
	Items    []T   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

func (r Request) Offset() int { return (r.Page - 1) * r.PageSize }
