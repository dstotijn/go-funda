package funda

import "net/url"

// House represents a house or real estate object on Funda.
type House struct {
	ID          int
	Address     string
	Price       string
	URL         url.URL
	ImageURL    url.URL
	SurfaceArea string
	Rooms       string
}
