package funda

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
)

const baseURL = "https://mobile.funda.io/api/v1"

type searchResultItem struct {
	ItemType int    `json:"ItemType"`
	GlobalID int    `json:"GlobalId"`
	Link     string `json:"Link"`
	Fotos    []foto `json:"Fotos"`
	Info     []info `json:"Info"`
}

type info struct {
	Line []houseResponseItemList `json:"Line"`
}

type foto struct {
	Link string `json:"Link"`
}

type searchResult []searchResultItem

type houseResponseItem struct {
	URL     string            `json:"URL"`
	List    []json.RawMessage `json:"List"`
	Section int               `json:"Section"`
}

type houseResponseItemList struct {
	Label string            `json:"Label"`
	Value string            `json:"Value"`
	Text  string            `json:"Text"`
	List  []json.RawMessage `json:"List"`
}

type houseResponse []houseResponseItem

// Client defines an HTTP client to the Funda API.
type Client struct {
	HTTPClient *http.Client
	BaseURL    string
	APIKey     string
}

// NewClient initialises and returns a new Client.
func NewClient(apiKey string) *Client {
	return &Client{
		HTTPClient: http.DefaultClient,
		BaseURL:    baseURL,
		APIKey:     apiKey,
	}
}

func (c *Client) newRequest(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("accepted_cookie_policy", "10")
	req.Header.Set("api_key", c.APIKey)
	req.Header.Set("User-Agent", "Funda/2.17.0 (com.funda.two; build:80; Android 25) okhttp/3.5.0")
	req.Header.Set("Cookie", "X-Stored-Data=null; expires=Fri, 31 Dec 9999 23:59:59 GMT; path=/; samesite=lax; httponly")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "nl-NL")

	return req, nil
}

func (c *Client) fundaSearchURL(searchOpts string, page, pageSize int) (*url.URL, error) {
	u, err := url.Parse(c.BaseURL + "/Aanbod/koop" + searchOpts)
	if err != nil {
		return nil, err
	}

	q := url.Values{}
	q.Set("page", strconv.Itoa(page))
	q.Set("pageSize", strconv.Itoa(pageSize))

	u.RawQuery = q.Encode()

	return u, nil
}

// Search does a house search request at the Funda API.
func (c *Client) Search(searchOpts string, page, pageSize int) ([]*House, error) {
	req, err := c.newRequest("GET", "", nil)
	if err != nil {
		return nil, fmt.Errorf("funda: could not create http request: %e", err)
	}

	u, err := c.fundaSearchURL(searchOpts, page, pageSize)
	if err != nil {
		return nil, fmt.Errorf("funda: could not parse search URL: %v", err)
	}
	req.URL = u

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("funda: could not execute http request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"funda: unexpected HTTP response code (%d) received",
			resp.StatusCode,
		)
	}

	houses, err := c.housesFromSearchResult(resp.Body)
	if err != nil {
		return nil, fmt.Errorf(
			"funda: could not parse houses from search result: %v",
			err,
		)
	}

	return houses, nil
}

func (c *Client) housesFromSearchResult(r io.Reader) ([]*House, error) {
	var result searchResult
	if err := json.NewDecoder(r).Decode(&result); err != nil {
		return nil, err
	}

	var houses []*House

	for _, item := range result {
		// Skip highlighted houses (ads).
		if item.ItemType != 1 {
			continue
		}

		if len(item.Fotos) < 1 {
			return nil, errors.New("result does not have photos")
		}

		if len(item.Info) < 4 {
			return nil, errors.New("result does not have enough info values")
		}

		for _, info := range item.Info {
			if len(info.Line) < 1 {
				return nil, errors.New("result does not have enough info lines")
			}
		}

		house := &House{
			ID:      item.GlobalID,
			Address: item.Info[0].Line[0].Text,
		}

		imageURL, err := url.Parse(item.Fotos[0].Link)
		if err != nil {
			return nil, err
		}
		house.ImageURL = *imageURL

		if err := c.populateHouseDetails(house, item.GlobalID); err != nil {
			log.Printf("Error: Could not get house (%v): %v", item.GlobalID, err)
			continue
		}

		houses = append(houses, house)
	}

	return houses, nil
}

func (c *Client) populateHouseDetails(house *House, globalID int) error {
	url := fmt.Sprintf("%v/Aanbod/Detail/Koop/%v", c.BaseURL, globalID)
	req, err := c.newRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("could not create http request: %e", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("funda: could not execute http request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(
			"funda: unexpected HTTP response code (%d) received",
			resp.StatusCode,
		)
	}

	if err := house.parseDetailsFromAPIResponse(resp.Body); err != nil {
		return fmt.Errorf(
			"funda: could not parse house from api response: %v",
			err,
		)
	}

	return nil
}

func (h *House) parseDetailsFromAPIResponse(r io.Reader) error {
	var houseResp houseResponse
	if err := json.NewDecoder(r).Decode(&houseResp); err != nil {
		return err
	}

	for _, item := range houseResp {
		if item.URL != "" {
			houseURL, err := url.Parse(item.URL)
			if err != nil {
				return err
			}
			h.URL = *houseURL
		}

		// Skip photos.
		if item.Section == 3 {
			continue
		}

		for _, l := range item.List {
			var list houseResponseItemList
			if err := json.Unmarshal(l, &list); err != nil {
				return err
			}
			if err := h.parseList(list); err != nil {
				return err
			}
		}
	}

	return nil
}

func (h *House) parseList(list houseResponseItemList) error {
	for _, l := range list.List {
		var list houseResponseItemList
		if err := json.Unmarshal(l, &list); err != nil {
			return err
		}
		h.parseList(list)
	}

	switch list.Label {
	case "Vraagprijs":
		h.Price = list.Value
	case "Wonen (= woonoppervlakte)":
		h.SurfaceArea = list.Value
	case "Aantal kamers":
		h.Rooms = list.Value
	}

	return nil
}
