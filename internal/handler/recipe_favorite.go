package handler

import (
	"recipe-server/internal/model"

	"gorm.io/gorm"
)

type recipeWithFavorite struct {
	model.Recipe
	IsFavorited bool `json:"is_favorited"`
}

func favoriteRecipeIDSet(db *gorm.DB, userID uint64, recipeIDs []uint64) map[uint64]bool {
	out := make(map[uint64]bool)
	if userID == 0 || len(recipeIDs) == 0 {
		return out
	}
	var ids []uint64
	db.Model(&model.Favorite{}).
		Where("user_id = ? AND recipe_id IN ?", userID, recipeIDs).
		Pluck("recipe_id", &ids)
	for _, id := range ids {
		out[id] = true
	}
	return out
}

func isRecipeFavorited(db *gorm.DB, userID, recipeID uint64) bool {
	if userID == 0 || recipeID == 0 {
		return false
	}
	var count int64
	db.Model(&model.Favorite{}).
		Where("user_id = ? AND recipe_id = ?", userID, recipeID).
		Count(&count)
	return count > 0
}

func recipesWithFavoriteFlags(db *gorm.DB, userID uint64, recipes []model.Recipe) []recipeWithFavorite {
	ids := make([]uint64, 0, len(recipes))
	for _, r := range recipes {
		ids = append(ids, r.ID)
	}
	favSet := favoriteRecipeIDSet(db, userID, ids)
	out := make([]recipeWithFavorite, 0, len(recipes))
	for _, r := range recipes {
		out = append(out, recipeWithFavorite{Recipe: r, IsFavorited: favSet[r.ID]})
	}
	return out
}
