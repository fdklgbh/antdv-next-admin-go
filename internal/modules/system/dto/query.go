package dto

type Query struct {
	Page      int    `form:"page" binding:"min=1,max=1000000"`
	PageSize  int    `form:"pageSize" binding:"min=1,max=10000"`
	Name      string `form:"name" binding:"max=128"`
	Key       string `form:"key" binding:"max=128"`
	Group     string `form:"group" binding:"max=64"`
	Code      string `form:"code" binding:"max=128"`
	TypeCode  string `form:"typeCode" binding:"max=128"`
	Label     string `form:"label" binding:"max=128"`
	Value     string `form:"value" binding:"max=128"`
	Status    string `form:"status" binding:"omitempty,oneof=enabled disabled success fail"`
	Username  string `form:"username" binding:"max=64"`
	Module    string `form:"module" binding:"max=128"`
	Action    string `form:"action" binding:"max=16"`
	IP        string `form:"ip" binding:"max=64"`
	Ext       string `form:"ext" binding:"max=32"`
	Storage   string `form:"storage" binding:"max=16"`
	StartTime string `form:"startTime" binding:"max=64"`
	EndTime   string `form:"endTime" binding:"max=64"`
}
type Page struct {
	List     any   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}
