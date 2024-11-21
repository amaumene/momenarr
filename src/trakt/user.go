package trakt

import (
	"encoding/json"
	"time"
)

type Gender string

const (
	GenderMale   Gender = `male`
	GenderFemale Gender = `female`
)

type VipStatus struct {
	Active bool
	EP     bool
}

type UserImages struct {
	Avatar *struct {
		Full string `json:"full"`
	} `json:"avatar"`
}

type User struct {
	baseIDs `json:"ids"`

	Username string      `json:"username"`
	Name     string      `json:"name"`
	Gender   Gender      `json:"gender"`
	Age      uint        `json:"age"`
	About    string      `json:"about"`
	Location string      `json:"location"`
	JoinedAt time.Time   `json:"joined_at"`
	Private  bool        `json:"private"`
	Vip      *VipStatus  `json:"-"`
	Images   *UserImages `json:"images"`
}

func (u *User) UnmarshalJSON(bytes []byte) error {
	type B User
	type A struct {
		B
		Vip   bool `json:"vip"`
		VipEp bool `json:"vip_ep"`
	}

	var a = new(A)
	err := json.Unmarshal(bytes, a)
	if err != nil {
		return nil
	}

	a.B.Vip = &VipStatus{
		Active: a.Vip,
		EP:     a.VipEp,
	}

	*u = User(a.B)
	return nil
}

type UserIterator struct{ Iterator }

func (u *UserIterator) User() (*User, error) {
	rcv := &User{}
	return rcv, u.Scan(rcv)
}
