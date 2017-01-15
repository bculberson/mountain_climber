package main

import "C"

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/lukeroth/gdal"
)

var datasets map[string]gdal.Dataset

func openDatasets(folder string) error {
	datasets = make(map[string]gdal.Dataset)
	files, err := ioutil.ReadDir(folder)
	if err != nil {
		return err
	}
	for _, f := range files {
		if strings.HasSuffix(f.Name(), "tif") {
			filename := fmt.Sprintf("%s/%s", folder, f.Name())
			ds, err := gdal.Open(filename, gdal.ReadOnly)
			if err != nil {
				return err
			}
			datasets[f.Name()] = ds
		}
	}
	return nil
}

func getElevation(key string, ds gdal.Dataset, lat float64, lng float64) (float64, error) {
	xoff := ds.GeoTransform()[0]
	a := ds.GeoTransform()[1]
	yoff := ds.GeoTransform()[3]
	e := ds.GeoTransform()[5]

	ur := xoff + float64(ds.RasterXSize())*a
	ll := yoff + float64(ds.RasterYSize())*e
	if lat >= ll && lat <= yoff && lng >= xoff && lng <= ur {
		log.Println(ds.GeoTransform())
		log.Printf("In bounds: %s, %d, %d, %d, %d\n", key, xoff, ur, yoff, ll)
		xflt := (yoff - lat) / a
		yflt := (xoff - lng) / e
		x := int(xflt)
		y := int(yflt)
		band := ds.RasterBand(1)
		log.Printf("x: %d, y: %d\n", x, y)
		eleLL := make([]uint16, 1)
		err := band.IO(gdal.Read, x, y, 1, 1, eleLL, 1, 1, 0, 0)
		if err != nil {
			return 0, err
		}
		eleLR := make([]uint16, 1)
		err = band.IO(gdal.Read, x+1, y, 1, 1, eleLR, 1, 1, 0, 0)
		if err != nil {
			eleLR = eleLL
		}
		eleUL := make([]uint16, 1)
		err = band.IO(gdal.Read, x, y+1, 1, 1, eleUL, 1, 1, 0, 0)
		if err != nil {
			eleUL = eleLL
		}
		eleUR := make([]uint16, 1)
		err = band.IO(gdal.Read, x+1, y+1, 1, 1, eleUR, 1, 1, 0, 0)
		if err != nil {
			eleUR = eleLL
		}
		dx := xflt - float64(x)
		dy := yflt - float64(y)
		log.Printf("%d, %d, %d, %d\n", eleLL[0], eleUL[0], eleLR[0], eleUR[0])
		ele := float64(eleLL[0])*(float64(1)-dx)*(float64(1)-dy) + float64(eleLR[0])*dx*(float64(1)-dy) + float64(eleUL[0])*(float64(1)-dx)*dy + float64(eleUR[0])*dx*dy
		return ele, nil
	}
	return 0, errors.New("out of bounds")
}

func elevations(w http.ResponseWriter, r *http.Request) {
	points := strings.Split(r.FormValue("points"), ",")
	if len(points)%2 != 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	lats := make([]float64, 0)
	lngs := make([]float64, 0)
	ix := 0
	for ix < len(points) {
		lat, err := strconv.ParseFloat(points[ix], 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		ix = ix + 1
		lng, err := strconv.ParseFloat(points[ix], 64)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		ix = ix + 1
		lats = append(lats, lat)
		lngs = append(lngs, lng)
	}
	lastDataset := ""
	results := make([]string, len(lats))
Lats:
	for ix, lat := range lats {
		lng := lngs[ix]
		if lastDataset != "" {
			ele, err := getElevation(lastDataset, datasets[lastDataset], lat, lng)
			if err == nil {
				results[ix] = fmt.Sprintf("%.2f", ele)
				continue Lats
			}

		}
		for key, ds := range datasets {
			ele, err := getElevation(key, ds, lat, lng)
			if err == nil {
				lastDataset = key
				results[ix] = fmt.Sprintf("%.2f", ele)
				continue Lats
			}
		}
		results[ix] = "-32768"
	}
	w.WriteHeader(http.StatusOK)
	output := strings.Join(results, ",")
	io.WriteString(w, output)
	return
}

func elevation(w http.ResponseWriter, r *http.Request) {
	lat, err := strconv.ParseFloat(r.FormValue("lat"), 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	lng, err := strconv.ParseFloat(r.FormValue("lng"), 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	log.Printf("lat: %d, lng %d", lat, lng)
	for key, ds := range datasets {
		ele, err := getElevation(key, ds, lat, lng)
		if err == nil {
			w.WriteHeader(http.StatusOK)
			output := fmt.Sprintf("%.2f", ele)
			io.WriteString(w, output)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
	return
}

func main() {
	flag.Parse()
	folder := flag.Arg(0)
	log.Printf("Folder with data: %s\n", folder)
	err := openDatasets(folder)
	if err != nil {
		panic(err)
	}

	http.HandleFunc("/v1/get_elevation", elevation)
	http.HandleFunc("/v1/get_elevations", elevations)
	http.ListenAndServe(":8000", nil)
}
