package main

import (
	"time"
)

type Photo struct {
	Id           uint64 `gorm:"column:photo_id"`
	FileName     string `gorm:"column:file_name"`
	UserId       uint64 `gorm:"column:user_id"`
	Description  string `gorm:"column:description"`
	Added        time.Time `gorm:"column:added"`
	AlbumId      *uint64 `gorm:"column:aid"`
	ModApproved  bool `gorm:"column:mod_approved"`
	Height       uint
	width        uint
	ISO2         uint `gorm:"column:country_iso2"`
	CONT         uint `gorm:"column:cont"`
	ThumbVersion uint16 `gorm:"column:thumb_version"`
	ThumbParams  string `gorm:"column:thumb_params"`
	HM64         string `gorm:"column:hm64"`
	Hidden       bool `gorm:"column:hidden"`
}
