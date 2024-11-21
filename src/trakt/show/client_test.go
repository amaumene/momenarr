package show

import (
	"context"
	"fmt"

	"github.com/jacklaaa89/trakt"
)

func ExampleRelated() {
	trakt.Key = "<client_id>"

	id := trakt.Slug("one-piece")

	it := Related(id, &trakt.ExtendedListParams{
		BasicListParams: trakt.BasicListParams{
			Context: context.Background(),
		},
	})

	for it.Next() {
		s, err := it.Show()
		if err != nil {
			continue
		}
		fmt.Printf("%v (%v)\n", s.Title, s.Year)
	}
	// Output:
	// One-Punch Man (2015)
	// Dragon Ball Z (1989)
	// Naruto (2002)
	// Dragon Ball (1986)
	// Steins;Gate (2011)
	// My Hero Academia (2016)
	// Fairy Tail (2009)
	// Dragon Ball Super (2015)
	// Samurai Champloo (2004)
	// Dragon Ball GT (1996)
}

func ExampleSeasons() {
	trakt.Key = "<client_id>"

	id := trakt.Slug("one-piece")
	sh, err := Get(id, nil)
	if err != nil {
		return
	}

	it := Seasons(id, &trakt.ExtendedListParams{
		BasicListParams: trakt.BasicListParams{
			Context: context.Background(),
		},
		Extended: trakt.ExtendedTypeEpisodes,
	})

	fmt.Printf("%v - (%v)\n", sh.Title, sh.Year)
	fmt.Println("========================")
	for it.Next() {
		s, err := it.Season()
		if err != nil {
			continue
		}
		title := fmt.Sprintf("Season %v", s.Number)
		if s.Number == 0 {
			title = "Specials"
		}

		fmt.Println(title)
		fmt.Println("========================")
		for _, ep := range s.Episodes {
			fmt.Printf("%v - %v\n", ep.Number, ep.Title)
		}
		fmt.Println("========================")
	}
	// Output:
	// One Piece - (1999)
	// ========================
	// Specials
	// ========================
	// 1 - One Piece: Defeat the Pirate Ganzack! (OVA 1)
	// 2 - One Piece: The Movie
	// 3 - Adventure in the Ocean's Navel
	// 4 - Clockwork Island Adventure
	// 5 - Jango's Dance Carnival
	// 6 - Chopper's Kingdom on the Island of Strange Animals
	// 7 - Soccer King of Dreams
	// 8 - Dead End Adventure
	// 9 - Open Upon the Great Sea! A Father's Huge, HUGE Dream!
	// 10 - Protect! The Last Great Performance
	// 11 - Curse of the Sacred Sword
	// 12 - Baseball King
	// ...
}
