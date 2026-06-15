package repository

// subj identifies the owner of a metadata row: a user XOR a band. Exactly
// one of the two ids is set. The schema keys annotations, resources, and
// practice events by subject, so the metadata operations are written once
// against subj and exposed as user- and band-specific public methods.
type subj struct {
	userID *uint
	bandID *uint
}

func userSubj(id uint) subj { return subj{userID: &id} }
func bandSubj(id uint) subj { return subj{bandID: &id} }

// scope returns a column filter selecting only this subject's rows (the
// other subject column is required to be NULL so the two layers never mix)
// together with its bind value.
func (s subj) scope() (string, uint) {
	if s.userID != nil {
		return "user_id = ? AND band_id IS NULL", *s.userID
	}
	return "band_id = ? AND user_id IS NULL", *s.bandID
}

// ownerScope filters owner-keyed tables (folders) to this subject, requiring
// the other owner column to be NULL so a user filter never matches a band row
// and vice versa.
func (s subj) ownerScope() (string, uint) {
	if s.userID != nil {
		return "owner_user_id = ? AND owner_band_id IS NULL", *s.userID
	}
	return "owner_band_id = ? AND owner_user_id IS NULL", *s.bandID
}
