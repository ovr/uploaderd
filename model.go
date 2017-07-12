package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"time"
)

type Album struct {
	Id          uint64 `gorm:"column:aid" json:"id"`
	UserId      uint64 `gorm:"column:user_id" json:"uid"`
	PhotosTotal uint32 `gorm:"column:photo_total"`
}

type Photo struct {
	Id           uint64    `gorm:"column:photo_id" json:"id,string"`
	Added        time.Time `gorm:"column:added" json:"created"`
	FileName     string    `gorm:"column:file_name" json:"path"`
	UserId       uint64    `gorm:"column:user_id" json:"uid,string"`
	AlbumId      *uint64   `gorm:"column:aid" json:"aid,string"`
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

type Info struct {
	Path string `json:"filename"`
	Size uint64 `json:"size,string"`
}

type AudioData struct {
	Data Info `json:"format"`
}

type Audio struct {
	Id      uint64 `gorm:"column:id" json:"id,string"`
	UserId  uint64 `gorm:"column:uid" json:"uid,string"`
	Path    string `gorm:"column:path"`
	Size    uint64 `gorm:"column:size"`
	Created string `gorm:"column:created" json:"created"`
}

func (this *AudioData) getAudioData(configFile string) {
	configJson, err := ioutil.ReadFile(configFile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = json.Unmarshal(configJson, &this)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// @todo Will be used in the feature
func (this Photo) getApiData() Photo {
	return this
}

func (this Audio) getApiData() Audio {
	return this
}
