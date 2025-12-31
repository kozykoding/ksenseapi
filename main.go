// Highest rated score for 73 version
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	APIKey  = "ak_55b4e28798ca491421030133dab7ec9482c298cc50e0c9bb"
	BaseURL = "https://assessment.ksensetech.com/api"
)

// Patient incoming JSON structure
type Patient struct {
	ID            string      `json:"patient_id"`
	Name          string      `json:"name"`
	Age           interface{} `json:"age"` // For unexpected types
	Temperature   interface{} `json:"temperature"`
	BloodPressure string      `json:"blood_pressure"`
}

type APIResponse struct {
	Data       []Patient `json:"data"`
	Pagination struct {
		HasNext bool `json:"hasNext"`
	} `json:"pagination"`
}

func main() {
	allPatients := fetchAllPatients()

	var highRisk []string
	var feverPatients []string
	var qualityIssues []string

	for _, p := range allPatients {
		totalScore, isInvalid, hasFever := processPatient(p)

		if isInvalid {
			qualityIssues = append(qualityIssues, p.ID)
		}
		if hasFever {
			feverPatients = append(feverPatients, p.ID)
		}
		if totalScore >= 4 {
			highRisk = append(highRisk, p.ID)
		}
	}

	submitAssessment(highRisk, feverPatients, qualityIssues)
}

func processPatient(p Patient) (int, bool, bool) {
	bpScore, bpInvalid := calculateBP(p.BloodPressure)
	temp, tempScore, tempInvalid := calculateTemp(p.Temperature)
	ageScore, ageInvalid := calculateAge(p.Age)

	isInvalid := bpInvalid || tempInvalid || ageInvalid
	hasFever := !tempInvalid && temp >= 99.6

	return (bpScore + tempScore + ageScore), isInvalid, hasFever
}

// Helper to parse BP "120/80"
func calculateBP(bpStr string) (int, bool) {
	parts := strings.Split(bpStr, "/")
	if len(parts) != 2 {
		return 0, true
	}

	sys, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	dia, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		return 0, true
	}

	// Risk Stages
	var sysStage, diaStage int

	if sys < 120 {
		sysStage = 1
	} else if sys <= 129 {
		sysStage = 2
	} else if sys <= 139 {
		sysStage = 3
	} else {
		sysStage = 4
	}
	if dia < 80 {
		diaStage = 1
	} else if dia <= 89 {
		diaStage = 3
	} else {
		diaStage = 4
	}

	// Return the higher of the two stages
	if sysStage > diaStage {
		return sysStage, false
	}
	return diaStage, false
}

func calculateTemp(t interface{}) (float64, int, bool) {
	val, err := getFloat(t)
	if err != nil {
		return 0, 0, true
	}
	// Normal temp = 0
	if val <= 99.5 {
		return val, 0, false
	}
	// Low fever = 1
	if val <= 100.9 {
		return val, 1, false
	}
	// Otherwise high fever = 2
	return val, 2, false
}

func calculateAge(a interface{}) (int, bool) {
	val, err := getInt(a)
	if err != nil {
		// Invalid/Missing Data = 0 points
		return 0, true
	}

	if val > 65 {
		// Over 65 = 2 points
		return 2, false
	} else if val >= 40 && val <= 65 {
		// 40-65 = 1 point
		return 1, false
	} else if val < 40 {
		// Under 40 = 1 point
		return 1, false
	}

	return 0, false
}

// Utility to handle mixed types (strings vs numbers in JSON)
func getFloat(unk interface{}) (float64, error) {
	switch v := unk.(type) {
	case float64:
		return v, nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("invalid")
	}
}

func getInt(unk interface{}) (int, error) {
	switch v := unk.(type) {
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("invalid")
	}
}

func fetchAllPatients() []Patient {
	var list []Patient
	page := 1
	client := &http.Client{}

	for {
		req, _ := http.NewRequest("GET", fmt.Sprintf("%s/patients?page=%d", BaseURL, page), nil)
		req.Header.Set("x-api-key", APIKey)

		resp, err := client.Do(req)
		if err != nil || resp.StatusCode >= 500 || resp.StatusCode == 429 {
			time.Sleep(1 * time.Second) // Simple retry logic
			continue
		}

		var apiRes APIResponse
		json.NewDecoder(resp.Body).Decode(&apiRes)
		resp.Body.Close()

		list = append(list, apiRes.Data...)
		if !apiRes.Pagination.HasNext {
			break
		}
		page++
	}
	return list
}

func submitAssessment(highRisk, fever, quality []string) {
	payload := map[string][]string{
		"high_risk_patients":  highRisk,
		"fever_patients":      fever,
		"data_quality_issues": quality,
	}
	body, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", BaseURL+"/submit-assessment", bytes.NewBuffer(body))
	req.Header.Set("x-api-key", APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, _ := client.Do(req)
	defer resp.Body.Close()

	resBody, _ := io.ReadAll(resp.Body)
	fmt.Println(string(resBody))
}
