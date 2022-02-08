// This file is autogenerated. DO NOT EDIT.

package model

import (
	"context"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/jxskiss/dbgen"
	"github.com/jxskiss/errors"
	"github.com/jxskiss/gopkg/v2/sqlutil"
	"gorm.io/gorm"
)

var _ context.Context
var _ time.Time
var _ proto.Message
var _ errors.ErrorGroup
var _ dbgen.Opt
var _ sqlutil.Bitmap
var _ gorm.DB

type Vehicle struct {
	Id                int64  `db:"id" gorm:"column:id;primaryKey"`                        // int(11)
	Brand             string `db:"brand" gorm:"column:brand"`                             // varchar(45)
	Model             string `db:"model" gorm:"column:model"`                             // varchar(45)
	ModelYear         int    `db:"model_year" gorm:"column:model_year"`                   // year(-1)
	Mileage           int    `db:"mileage" gorm:"column:mileage"`                         // int(9) UNSIGNED
	Color             string `db:"color" gorm:"column:color"`                             // varchar(45)
	VehicleTypeId     int    `db:"vehicle_type_id" gorm:"column:vehicle_type_id"`         // int(11)
	CurrentLocationId int    `db:"current_location_id" gorm:"column:current_location_id"` // int(11)
}

type VehicleList []*Vehicle

func (p VehicleList) ToIdMap() map[int64]*Vehicle {
	out := make(map[int64]*Vehicle, len(p))
	for _, x := range p {
		out[x.Id] = x
	}
	return out
}

func (p VehicleList) PluckIds() []int64 {
	out := make([]int64, 0, len(p))
	for _, x := range p {
		out = append(out, x.Id)
	}
	return out
}