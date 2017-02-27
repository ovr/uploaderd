package main

import (
	"time"
)

type Album struct {
	Id          uint64 `gorm:"column:aid" json:"id"`
	UserId      uint64 `gorm:"column:user_id" json:"uid"`
	PhotosTotal uint32 `gorm:"column:photo_total"`
}

type Photo struct {
	Id           uint64    `gorm:"column:photo_id" json:"id"`
	Added        time.Time `gorm:"column:added" json:"created"`
	FileName     string    `gorm:"column:file_name" json:"path"`
	UserId       uint64    `gorm:"column:user_id" json:"uid"`
	AlbumId      *uint64   `gorm:"column:aid" json:"aid"`
	Description  string    `gorm:"column:description" json:"-"`
	ModApproved  bool      `gorm:"column:mod_approved" json:"-"`
	Height       uint64    `json:"height"`
	Width        uint64    `json:"width"`
	ISO2         uint      `gorm:"column:country_iso2" json:"-"`
	CONT         uint      `gorm:"column:cont" json:"-"`
	ThumbVersion uint16    `gorm:"column:thumb_version" json:"version"`
	ThumbParams  string    `gorm:"column:thumb_params" json:"-"`
	HM64         string    `gorm:"column:hm64" json:"-"`
	Hidden       bool      `gorm:"column:hidden" json:"-"`
}

// @todo Will be used in the feature
func (this Photo) getApiData() Photo {
	return this
}
