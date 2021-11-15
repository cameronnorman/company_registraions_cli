package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gocolly/colly"
	"github.com/urfave/cli/v2"
)

type CompanyRegistration struct {
	RegNo      string     `json:"regno"`
	Date       *time.Time `json:"date"`
	Address    string     `json:"address"`
	City       string     `json:"city"`
	PostalCode string     `json:"postalCode"`
	Name       string     `json:"name"`
}

type filterDate struct {
	day   int
	month string
	year  int
}

func extractCompanyNumber(text string) (string, error) {
	r := regexp.MustCompile(`.*:\s(.*)\n.*`)
	matches := r.FindStringSubmatch(text)
	if len(matches) > 0 {
		return matches[1], nil
	}

	return "", fmt.Errorf("unable to extract company registration number: %s", text)
}

func extractCompanyRegistrationDate(text string) (*time.Time, error) {
	r := regexp.MustCompile(`.*Bekannt gemacht am:(.*)Uhr`)
	matches := r.FindStringSubmatch(text)
	if len(matches) > 0 {
		layout := "02.01.2006 15:04"
		t, err := time.Parse(layout, strings.TrimSpace(matches[1]))
		if err != nil {
			return nil, fmt.Errorf("unable to extract company reg date: %s", err.Error())
		}

		return &t, nil
	}

	return nil, errors.New("unable to extract company registration date")
}

func extractCompanyName(text string) (string, error) {
	sections := strings.Split(text, ",")
	parts := strings.Split(sections[0], ": ")

	return parts[1], nil
}

func extractCompanyAddress(text string) (string, error) {
	sections := strings.Split(text, ",")

	if len(sections[2]) > 35 {
		fmt.Println(text)
	}

	return sections[2], nil
}

func extractCity(text string) (string, error) {
	sections := strings.Split(text, ",")
	return sections[1], nil
}

func extractPostalCode(text string) (string, error) {
	r := regexp.MustCompile(`.*(\d{5}).*`)
	matches := r.FindStringSubmatch(text)
	if len(matches) > 0 {
		return matches[1], nil
	}

	return "", fmt.Errorf("unable to extract company postal code: %v", text)
}

func collectRegistrations(startDate time.Time, endDate time.Time) []CompanyRegistration {
	c := colly.NewCollector()
	registrations := []CompanyRegistration{}

	c.OnHTML("li>a[href]", func(e *colly.HTMLElement) {
		href := e.Attr("href")
		r := regexp.MustCompile(`.*'rb_id=(.*)\&.*`)
		matches := r.FindStringSubmatch(href)
		if len(matches) > 0 {
			regID := matches[1]
			regURL := fmt.Sprintf("https://www.handelsregisterbekanntmachungen.de/skripte/hrb.php?rb_id=%s&land_abk=bw", regID)
			c.Visit(regURL)
		}
	})

	c.OnHTML("font", func(e *colly.HTMLElement) {
		lines := []string{}
		e.ForEach("tr", func(count int, ee *colly.HTMLElement) {
			lines = append(lines, ee.Text)
		})

		if len(lines) > 0 {
			reg := CompanyRegistration{}
			courtFileNumber, err := extractCompanyNumber(lines[0])
			if err != nil {
				log.Println(err.Error())
			}
			reg.RegNo = strings.TrimSpace(courtFileNumber)

			companyRegistrationDate, err := extractCompanyRegistrationDate(lines[0])
			if err != nil {
				log.Println(err.Error())
			}
			reg.Date = companyRegistrationDate

			companyName, err := extractCompanyName(lines[5])
			if err != nil {
				log.Println(err.Error())
			}
			reg.Name = strings.TrimSpace(companyName)

			companyAddress, err := extractCompanyAddress(lines[5])
			if err != nil {
				log.Println(err.Error())
			}
			reg.Address = strings.TrimSpace(companyAddress)

			companyCity, err := extractCity(lines[5])
			if err != nil {
				log.Println(err.Error())
			}
			reg.City = strings.TrimSpace(companyCity)

			postalCode, err := extractPostalCode(lines[5])
			if err != nil {
				log.Println(err.Error())
			}
			reg.PostalCode = strings.TrimSpace(postalCode)

			registrations = append(registrations, reg)
		}
	})

	data := map[string]string{
		"suchart":      "uneingeschr",
		"button":       "Suche+starten",
		"vt":           fmt.Sprintf("%d", startDate.Day()),
		"vm":           fmt.Sprintf("%d", (startDate.Month())),
		"vj":           fmt.Sprintf("%d", startDate.Year()),
		"bt":           fmt.Sprintf("%d", endDate.Day()),
		"bm":           fmt.Sprintf("%d", (endDate.Month())),
		"bj":           fmt.Sprintf("%d", endDate.Year()),
		"land":         "",
		"gericht":      "",
		"gericht_name": "",
		"seite":        "",
		"l":            "",
		"r":            "",
		"all":          "false",
		"rubrik":       "",
		"az":           "",
		"gegenstand":   "0",
		"order":        "4",
	}

	c.Post("https://www.handelsregisterbekanntmachungen.de/?aktion=suche#Ergebnis", data)

	return registrations
}

func main() {
	app := &cli.App{
		Name:  "Company Registrations fetcher",
		Usage: "Fetches German company registrations",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "output",
				Value: "csv",
				Usage: "Specify which output format you want. (CSV and jsonl supported)",
			},
			&cli.StringFlag{
				Name:  "start_date",
				Value: fmt.Sprintf("%d-%d-%d", time.Now().Year(), int(time.Now().Month()), time.Now().Day()),
				Usage: "Specify the start date you would like to use (YYYY-mm-dd)",
			},
			&cli.StringFlag{
				Name:  "end_date",
				Value: fmt.Sprintf("%d-%d-%d", time.Now().Year(), int(time.Now().Month()), time.Now().Day()),
				Usage: "Specify the end date you would like to use (YYYY-mm-dd)",
			},
		},
		Action: func(c *cli.Context) error {
			layout := "2006-01-02"
			startDate, err := time.Parse(layout, strings.TrimSpace(c.String("start_date")))
			if err != nil {
				return fmt.Errorf("unable to parse start date parameter: %s", err.Error())
			}

			endDate, err := time.Parse(layout, strings.TrimSpace(c.String("end_date")))
			if err != nil {
				return fmt.Errorf("unable to parse end date parameter: %s", err.Error())
			}

			registrations := collectRegistrations(startDate, endDate)
			if c.String("output") == "jsonl" {
				for _, r := range registrations {
					m, _ := json.Marshal(r)
					fmt.Printf("%s\n", string(m))
				}

				return nil
			}

			if c.String("output") == "csv" {
				fmt.Println("RegNo;Date;Name;Address;City;PostalCode")
				for _, r := range registrations {
					fmt.Printf("%s;%v;%s;%s;%s;%s\n", r.RegNo, r.Date, r.Name, r.Address, r.City, r.PostalCode)
				}
				return nil
			}

			return errors.New("output not supported")
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
