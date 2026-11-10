package main

import (
	"image/color"

	"github.com/yohamta/donburi"
)

type PositionData struct {
	X float64
	Y float64
}

var Position = donburi.NewComponentType[PositionData]()

type RotationData struct {
	Angle float64
}

var Rotation = donburi.NewComponentType[RotationData]()

type ColorData struct {
	Color color.Color
}

var Color = donburi.NewComponentType[ColorData]()

type VelocityData struct {
	X float64
	Y float64

	Rotation float64
}

var Velocity = donburi.NewComponentType[VelocityData]()

var Clickable = donburi.NewTag()
