package trakt

import (
	"encoding/json"
	"time"
)

type Department string

const (
	DepartmentProduction       Department = "production"
	DepartmentArt              Department = "art"
	DepartmentCrew             Department = "crew"
	DepartmentCostumeAndMakeUp Department = "costume & make-up"
	DepartmentDirecting        Department = "directing"
	DepartmentWriting          Department = "writing"
	DepartmentSound            Department = "sound"
	DepartmentCamera           Department = "camera"
	DepartmentVisualEffects    Department = "visual effects"
	DepartmentLighting         Department = "lighting"
	DepartmentEditing          Department = "editing"
)

type Person struct {
	MediaIDs `json:"ids"`

	Name       string    `json:"name"`
	Biography  string    `json:"biography"`
	Birthday   time.Time `json:"-"`
	Death      time.Time `json:"-"`
	Homepage   string    `json:"homepage"`
	Birthplace string    `json:"birthplace"`
}

func (p *Person) UnmarshalJSON(bytes []byte) error {
	type B Person
	type A struct {
		B
		Birthday string `json:"birthday"`
		Death    string `json:"death"`
	}

	var a = new(A)
	err := json.Unmarshal(bytes, a)
	if err != nil {
		return err
	}

	var pErr error
	if len(a.Birthday) > 0 {
		a.B.Birthday, pErr = time.Parse(`2006-01-02`, a.Birthday)
		if pErr != nil {
			return pErr
		}
	}

	if len(a.Death) > 0 {
		a.B.Death, pErr = time.Parse(`2006-01-02`, a.Death)
		if pErr != nil {
			return pErr
		}
	}

	*p = Person(a.B)
	return nil
}

type CastEntry struct {
	Characters []string `json:"characters"`
	Episodes   *int64   `json:"episode_count"`
	Person     `json:"person"`
}

type CrewEntry struct {
	Jobs     []string `json:"jobs"`
	Episodes *int64   `json:"episode_count"`
	Person   `json:"person"`
}

type CastAndCrew struct {
	Cast []*CastEntry                `json:"cast"`
	Crew map[Department][]*CrewEntry `json:"crew"`
}

type CrewCredit struct {
	topLevelMediaElement
	Jobs []string `json:"jobs"`
}

func (b *CrewCredit) UnmarshalJSON(bytes []byte) error {
	type A CrewCredit
	var a = new(A)
	err := json.Unmarshal(bytes, a)
	if err != nil {
		return err
	}

	switch {
	case a.Show != nil:
		a.Type = TypeShow
	case a.Movie != nil:
		a.Type = TypeMovie
	}

	*b = CrewCredit(*a)
	return nil
}

type CastCredit struct {
	topLevelMediaElement
	Characters []string `json:"characters"`
}

func (b *CastCredit) UnmarshalJSON(bytes []byte) error {
	type A CastCredit
	var a = new(A)
	err := json.Unmarshal(bytes, a)
	if err != nil {
		return err
	}

	switch {
	case a.Show != nil:
		a.Type = TypeShow
	case a.Movie != nil:
		a.Type = TypeMovie
	}

	*b = CastCredit(*a)
	return nil
}

type Credits struct {
	Cast []*CastCredit                `json:"cast"`
	Crew map[Department][]*CrewCredit `json:"crew"`
}
