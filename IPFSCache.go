package main

import (
	"fmt"
	"log"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/scylladb/go-set/iset"
	"github.com/unki2aut/go-mpd"
)

type Matcher struct {
	QualityString string
	Quality       int
	Suffix        string
	Bandwidth     float64
}

type IPFSCache struct {
	IPFSCachedSegments map[uint64]*iset.Set
	mpdTree            mpd.MPD
	SegmentDuration    time.Duration
	URLMatcher         map[string]Matcher
	LatestProgress     map[string]uint64
}

func NewIPFSCache(m *mpd.MPD) *IPFSCache {
	c := new(IPFSCache)
	c.IPFSCachedSegments = make(map[uint64]*iset.Set)
	c.URLMatcher = make(map[string]Matcher)
	c.LatestProgress = make(map[string]uint64)
	c.mpdTree = *m
	c.PrepareURLMatcher()
	return c
}

func Stoi(s string) int {
	r, _ := strconv.Atoi(s)
	return r
}

func (c *IPFSCache) PrepareURLMatcher() {
	for _, p := range c.mpdTree.Period {
		for _, adapt := range p.AdaptationSets {
			for i, _ := range adapt.Representations {
				representation := &adapt.Representations[i]
				pos := strings.LastIndex(*representation.SegmentTemplate.Media, "$Number$")
				c.SegmentDuration = time.Duration(*representation.SegmentTemplate.Duration / *representation.SegmentTemplate.Timescale) * (time.Second / time.Nanosecond)

				c.URLMatcher[(*representation.SegmentTemplate.Media)[:pos]] = Matcher{
					QualityString: *representation.ID,
					Quality:       Stoi(*representation.ID),
					Suffix:        (*representation.SegmentTemplate.Media)[pos+len("$Number$"):],
					Bandwidth:     float64(*representation.Bandwidth),
				}
			}
		}
	}
}

func (c *IPFSCache) AlreadyCachedUrl(url string) bool {
	segment, quality := c.ParseSegmentQuality(url)
	return c.AlreadyCached(segment, quality)
}

func (c *IPFSCache) AlreadyCached(segment uint64, quality int) bool {
	if segment == 0 || c.IPFSCachedSegments[segment] == nil {
		return false
	}

	return c.IPFSCachedSegments[segment].Has(quality)
}

func (c *IPFSCache) GreatestQuality(segment uint64) int {
	max := 0
	if c.IPFSCachedSegments[segment] == nil {
		return max
	}
	for _, cachedQuality := range c.IPFSCachedSegments[segment].List() {
		if cachedQuality > max {
			max = cachedQuality
		}
	}
	return max
}

func (c *IPFSCache) QualitysBandwidth(quality int) float64 {
	for _, value := range c.URLMatcher {
		if value.Quality == quality {
			return value.Bandwidth
		}
	}
	return 0.0
}

func (c *IPFSCache) FormUrlBySegmentQuality(segment uint64, quality int) string {
	for prefix, value := range c.URLMatcher {
		if value.Quality == quality {
			return prefix + fmt.Sprint(segment) + value.Suffix
		}
	}
	return ""
}

func (c *IPFSCache) ParseSegmentQuality(url string) (uint64, int) {
	var quality int
	var segment uint64
	url = path.Base(url)
	for key, value := range c.URLMatcher {
		if strings.HasPrefix(url, key) {
			url = strings.TrimPrefix(url, key)
			segment = uint64(Stoi(strings.TrimSuffix(url, value.Suffix)))
			quality = value.Quality
		}
	}
	return segment, quality
}

func (c *IPFSCache) AddRecordFromURL(url string, clientID string) error {
	segment, quality := c.ParseSegmentQuality(url)
	if segment != 0 {
		c.LatestProgress[clientID] = segment
		c.AddRecord(segment, quality, clientID)
	}
	return nil
}

func (c *IPFSCache) AddRecord(segment uint64, quality int, clientID string) {
	if _, ok := c.IPFSCachedSegments[segment]; !ok {
		c.IPFSCachedSegments[segment] = iset.New()
	}

	if segment != 0 {
		c.LatestProgress[clientID] = segment
	}

	c.IPFSCachedSegments[segment].Add(quality)
	//	log.Println("Add segment", number, ":", representationID)
}

func (c *IPFSCache) Latest(clientID string) (*iset.Set, uint64) {
	latest := c.LatestProgress[clientID] + 1
	if val, ok := c.IPFSCachedSegments[latest]; !ok {
		return &iset.Set{}, latest
	} else {
		return val, latest
	}
}

func (c *IPFSCache) Print() {
	log.Println("x")
}
