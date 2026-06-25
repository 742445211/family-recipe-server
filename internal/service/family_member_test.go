package service

import (
	"testing"

	"recipe-server/internal/model"
	"recipe-server/internal/testutil"
)

func TestIsFamilyMember(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)

	if !IsFamilyMember(db, userID, familyID) {
		t.Fatal("seed user should be family member")
	}
	if IsFamilyMember(db, userID, 99999) {
		t.Fatal("should not be member of unknown family")
	}
	if IsFamilyMember(db, 99999, familyID) {
		t.Fatal("unknown user should not be member")
	}
}

func TestResolveJWTFamilyID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)

	if got := ResolveJWTFamilyID(db, userID, familyID); got != familyID {
		t.Fatalf("want %d, got %d", familyID, got)
	}
	if got := ResolveJWTFamilyID(db, userID, familyID+100); got != 0 {
		t.Fatalf("non-member claim should be 0, got %d", got)
	}
	if got := ResolveJWTFamilyID(db, userID, 0); got != 0 {
		t.Fatalf("zero claim should be 0, got %d", got)
	}
}

func TestResolveEffectiveFamilyIDFallsBackToCurrentFamily(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)

	if got := ResolveEffectiveFamilyID(db, userID, 0); got != familyID {
		t.Fatalf("JWT family_id=0 时应回退 current_family_id, want %d got %d", familyID, got)
	}
}

func TestAssertFamilyMember(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userID, familyID := testutil.SeedUserAndFamily(t, db)

	if err := AssertFamilyMember(db, userID, familyID); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if err := AssertFamilyMember(db, userID, 0); err == nil {
		t.Fatal("family 0 should fail")
	}
	other := model.User{OpenID: "other", Nickname: "other"}
	db.Create(&other)
	if err := AssertFamilyMember(db, other.ID, familyID); err == nil {
		t.Fatal("non-member should fail")
	}
}
