package funda

import (
	"bufio"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
)

func TestParseDetailsFromAPIResponse(t *testing.T) {
	searchFile, err := os.Open("test_data/funda_search_response.json")
	if err != nil {
		t.Fatal(err)
	}

	houseFile, err := os.Open("test_data/funda_house_response.json")
	if err != nil {
		t.Fatal(err)
	}

	files := []io.Reader{searchFile, houseFile}

	reqCount := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reader := bufio.NewReader(files[reqCount])
		reader.WriteTo(w)
		reqCount++
	}))
	defer ts.Close()

	fundaClient := NewClient("foobar")
	fundaClient.BaseURL = ts.URL

	exp := House{
		ID:          4094475,
		Address:     "Buiksloterbreek 65",
		Price:       "€ 400.000 k.k.",
		URL:         parseURL("https://www.funda.nl/40443683"),
		ImageURL:    parseURL("https://cloud.funda.nl/valentina_media/090/700/422_720x480.jpg"),
		SurfaceArea: "68 m²",
		Rooms:       "3 kamers (1 slaapkamer)",
	}

	got, err := fundaClient.Search("", 0, 0)
	if err != nil {
		t.Fatalf("Got: %v, expected %v", err, nil)
	}

	if len(got) != 1 {
		t.Fatalf("Got: %v houses, expected %v", len(got), 1)
	}

	if *got[0] != exp {
		t.Fatalf("Got: %+v, expected %+v", *got[0], exp)
	}
}

func parseURL(s string) url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}

	return *u
}
