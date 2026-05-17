package dto

import "time"

type UserInfo struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Language    string    `json:"language"`
	Menus       []string  `json:"menus"`
	Nodes       []string  `json:"nodes"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
}

type UserCreate struct {
	Name        string   `json:"name" validate:"required"`
	Password    string   `json:"password" validate:"required"`
	Status      string   `json:"status"`
	Language    string   `json:"language"`
	Menus       []string `json:"menus"`
	Nodes       []string `json:"nodes"`
	Description string   `json:"description"`
}

type UserUpdate struct {
	ID          uint     `json:"id" validate:"required"`
	Status      string   `json:"status"`
	Language    string   `json:"language"`
	Menus       []string `json:"menus"`
	Nodes       []string `json:"nodes"`
	Description string   `json:"description"`
}

type UserUpdatePassword struct {
	ID       uint   `json:"id" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type UserSearch struct {
	SearchWithPage
	Status string `json:"status"`
}
