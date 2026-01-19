package redshift

import (
	"time"

	"github.com/nathan-osman/go-sunrise"
)

// Solar interpolates a temperature based on the provided longitude and
// latitude, interpolating between tempNight and tempDay when the sun is between
// elevationNight and elevationDay.
func Solar(now time.Time, lat, lng float64, elevationNight, elevationDay float64, tempNight, tempDay Temperature) Temperature {
	var progress float64
	switch elevation := sunrise.Elevation(lat, lng, time.Now()); {
	case elevation < elevationNight:
		progress = 0
	case elevation >= elevationDay:
		progress = 1
	default:
		progress = (elevationNight - elevation) / (elevationNight - elevationDay)
	}
	return Temperature((1-progress)*float64(tempNight) + progress*float64(tempDay))
}
