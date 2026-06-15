package repository

import (
	"testing"

	"github.com/jwhumphries/bandwidth/internal/model"
)

func TestCreateAndListBandSongs(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	band, _ := repo.CreateBand(alice.ID, "Band")

	song, err := repo.CreateBandSong(band.ID, "Wonderwall", "Oasis")
	if err != nil || song.OwnerBandID == nil || *song.OwnerBandID != band.ID {
		t.Fatalf("CreateBandSong: %+v (%v)", song, err)
	}

	items, err := repo.SongsForBand(band.ID)
	if err != nil || len(items) != 1 {
		t.Fatalf("SongsForBand: %v (%v)", items, err)
	}
	if items[0].Title != "Wonderwall" || items[0].Status != model.StatusNotLearned {
		t.Errorf("band song item = %+v", items[0])
	}

	// The band layer status is independent of any member's personal layer.
	status := model.StatusLearned
	if err := repo.UpsertBandAnnotation(song.ID, band.ID, &status, nil); err != nil {
		t.Fatalf("UpsertBandAnnotation: %v", err)
	}
	notes := "key of F#m"
	if err := repo.UpsertBandAnnotation(song.ID, band.ID, nil, &notes); err != nil {
		t.Fatal(err)
	}
	ann, err := repo.BandAnnotationForSong(song.ID, band.ID)
	if err != nil || ann.Status != model.StatusLearned || ann.Notes != "key of F#m" {
		t.Errorf("band annotation = %+v (%v)", ann, err)
	}
	// A member's personal annotation on the same band song is separate.
	personal := model.StatusLearning
	if err := repo.UpsertAnnotation(song.ID, alice.ID, &personal, nil); err != nil {
		t.Fatal(err)
	}
	items, _ = repo.SongsForBand(band.ID)
	if items[0].Status != model.StatusLearned {
		t.Errorf("band layer status leaked to/from user: %q", items[0].Status)
	}
}

func TestSongVisibleToUser(t *testing.T) {
	repo := testRepo(t)
	alice, _ := repo.CreateUser("alice", "alice@example.com", "h")
	bob, _ := repo.CreateUser("bob", "bob@example.com", "h")
	band, _ := repo.CreateBand(alice.ID, "Band")
	_ = repo.AddMember(band.ID, bob.ID, model.RoleViewer)
	bandSong, _ := repo.CreateBandSong(band.ID, "Shared", "Artist")
	ownSong, _ := repo.CreateSong(alice.ID, "Mine", "Artist")

	// Members (alice creator, bob viewer) can see the band song.
	for _, u := range []uint{alice.ID, bob.ID} {
		if _, err := repo.SongVisibleToUser(bandSong.ID, u); err != nil {
			t.Errorf("member %d cannot see band song: %v", u, err)
		}
	}
	// A non-member cannot.
	carol, _ := repo.CreateUser("carol", "carol@example.com", "h")
	if _, err := repo.SongVisibleToUser(bandSong.ID, carol.ID); err == nil {
		t.Error("non-member sees band song")
	}
	// Owner sees their own song; others don't.
	if _, err := repo.SongVisibleToUser(ownSong.ID, alice.ID); err != nil {
		t.Errorf("owner cannot see own song: %v", err)
	}
	if _, err := repo.SongVisibleToUser(ownSong.ID, bob.ID); err == nil {
		t.Error("non-owner sees personal song")
	}

	// SongForBand is band-scoped.
	if _, err := repo.SongForBand(bandSong.ID, band.ID); err != nil {
		t.Errorf("SongForBand(own band): %v", err)
	}
	if _, err := repo.SongForBand(ownSong.ID, band.ID); err == nil {
		t.Error("SongForBand returned a personal song")
	}
}
