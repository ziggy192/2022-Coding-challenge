package main

type ApiResponse struct {
	Data  *StateResponse `json:"data"`
	Error *ErrorResponse `json:"error"`
}

type StateResponse struct {
	Incidents []*Incident
	Officers  []*Officer
}

type Incident struct {
	ID        int      `json:"id"`
	CodeName  string   `json:"codeName"`
	Loc       Location `json:"loc"`
	OfficerId int      `json:"officerID,omitempty"`
	Officer   *Officer `json:"-"`
}

type Location struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Officer struct {
	ID        int       `json:"id"`
	BadgeName string    `json:"badgeName"`
	Loc       Location  `json:"loc"`
	Incident  *Incident `json:"-"`
}

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
