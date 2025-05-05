package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"time"
)

type Biatlon struct {
	Laps        int    `json:"laps"`
	LapLen      int    `json:"lapLen"`
	PenaltyLen  int    `json:"penaltyLen"`
	FiringLines int    `json:"firingLines"`
	Start       string `json:"start"`
	StartDelta  string `json:"startDelta"`
}

type Biatlonist struct {
	ID                string
	HeIsDisqualified  bool
	HeCantContinue    bool
	StartTime         string
	TotalTime         string
	DrawingLots       string
	LapsList          []LapData
	PenaltyList       []PenaltyData
	TotalHits         int
	CountHitsOnTarget int
	LastCountMiss     int
}

type LapData struct {
	TotalTime string
	TimeOut   string
	Speed     float64
}

type PenaltyData struct {
	TimeIn    string
	TimeOut   string
	TotalTime string
	Speed     float64
}

func main() {
	biatlon, err := getConfig()
	biatlonistMap := make(map[string]*Biatlonist)
	if err != nil {
		fmt.Println(err)
	}
	file, err := os.Create("output")
	if err != nil {
		fmt.Println(err)
	}
	defer file.Close()

	getIvents(file, biatlonistMap, biatlon)
	biatlonistList := SortedMap(biatlonistMap)
	fmt.Fprintln(file, "")
	for _, biatlonist := range biatlonistList {
		formatedPrint(file, biatlonist, biatlon)
	}

}

func formatedPrint(w io.Writer, biatlonist Biatlonist, biatlon *Biatlon) {
	if biatlonist.HeIsDisqualified {
		fmt.Fprintf(w, "[NotStarted] %s {,} {,} 0/0\n",
			biatlonist.ID,
		)
	} else if biatlonist.HeCantContinue {
		fmt.Fprintf(w, "[NotFinished] %s ",
			biatlonist.ID,
		)
		printLapsList(w, biatlonist.LapsList, biatlon)
		fmt.Fprintf(w, " ")
		printPenaltyList(w, biatlonist.PenaltyList)
		fmt.Fprintf(w, " %d/%d\n",
			biatlonist.CountHitsOnTarget,
			biatlonist.TotalHits,
		)
	} else {
		fmt.Fprintf(w, "[%s] %s ",
			biatlonist.TotalTime,
			biatlonist.ID,
		)
		printLapsList(w, biatlonist.LapsList, biatlon)
		fmt.Fprintf(w, " ")
		printPenaltyList(w, biatlonist.PenaltyList)
		fmt.Fprintf(w, " %d/%d\n",
			biatlonist.CountHitsOnTarget,
			biatlonist.TotalHits,
		)
	}

}
func truncateFloat(f float64, digits int) float64 {
	pow := math.Pow(10, float64(digits))
	return math.Trunc(f*pow) / pow
}

func printLapsList(w io.Writer, lapsList []LapData, biatlon *Biatlon) {
	if biatlon.Laps > 1 {
		fmt.Fprintf(w, "[")
	}
	for i, lap := range lapsList {
		fmt.Fprintf(w, "{%s, %.3f}", lap.TotalTime, truncateFloat(lap.Speed, 3))
		if i != biatlon.Laps-1 {
			fmt.Fprintf(w, ", ")
		}
	}
	dop := biatlon.Laps - len(lapsList)
	for i := 0; i < dop; i++ {
		fmt.Fprintf(w, "{,}")
		if i < dop-1 {
			fmt.Fprintf(w, ", ")
		}
	}
	if biatlon.Laps > 1 {
		fmt.Fprintf(w, "]")
	}
}

func printPenaltyList(w io.Writer, penaltyList []PenaltyData) {
	if len(penaltyList) > 1 {
		fmt.Fprintf(w, "[")
	}
	if len(penaltyList) == 0 {
		fmt.Fprintf(w, "{,}")
	}
	for i, pen := range penaltyList {

		fmt.Fprintf(w, "{%s, %.3f}", pen.TotalTime, truncateFloat(pen.Speed, 3))
		if i != len(penaltyList)-1 {
			fmt.Fprintf(w, ", ")
		}
	}
	if len(penaltyList) > 1 {
		fmt.Fprintf(w, "]")
	}
}

func SortedMap(biatlonistMap map[string]*Biatlonist) []Biatlonist {
	biatlonisList := make([]Biatlonist, 0)
	for _, v := range biatlonistMap {
		biatlonisList = append(biatlonisList, *v)
	}
	sort.Slice(biatlonisList, func(i, j int) bool {
		t1, _ := time.Parse("15:04:05.000", biatlonisList[i].TotalTime)
		t2, _ := time.Parse("15:04:05.000", biatlonisList[j].TotalTime)
		return t1.Before(t2)
	})
	return biatlonisList
}

func getIvents(w io.Writer, biatlonistMap map[string]*Biatlonist, biatlon *Biatlon) {
	file, err := os.Open("events")
	if err != nil {
		fmt.Println(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		sLine := strings.Split(line, " ")
		formatedEvent(w, biatlonistMap, biatlon, sLine)
	}
}

func formatedEvent(w io.Writer, biatlonistMap map[string]*Biatlonist, biatlon *Biatlon, line []string) {
	switch line[1] {
	case "1": // регистрация
		fmt.Fprintf(w, "%s The competitor(%s) registered\n", line[0], line[2])
		biatlonistMap[line[2]] = CreateBiatlonist(line)
	case "2": // жребий на старт
		fmt.Fprintf(w, "%s The start time for the competitor(%s) was set by a draw to %s\n", line[0], line[2], line[3])
		biatlonist := biatlonistMap[line[2]]
		biatlonist.DrawingLots = line[3]
	case "3": // на стартовой линии
		fmt.Fprintf(w, "%s The competitor(%s) is on the start line\n", line[0], line[2])
	case "4": // стартовал
		biatlonist := biatlonistMap[line[2]]
		err := checkStartBiatlonist(biatlon, biatlonist, line)
		if err != nil {
			fmt.Fprintf(w, "%s The competitor(%s) is disqualified\n", line[0], line[2])
		} else {
			fmt.Fprintf(w, "%s The competitor(%s) has started\n", line[0], line[2])
		}
	case "5": // зашел на огневой рубеж
		biatlonist := biatlonistMap[line[2]]
		biatlonist.TotalHits += 5
		biatlonist.LastCountMiss = 5
		fmt.Fprintf(w, "%s The competitor(%s) is on the firing range(%s)\n", line[0], line[2], line[3])
	case "6": // попадание по мишени
		biatlonist := biatlonistMap[line[2]]
		biatlonist.CountHitsOnTarget += 1
		biatlonist.LastCountMiss -= 1
		fmt.Fprintf(w, "%s The target(%s) has been hit by competitor(%s)\n", line[0], line[3], line[2])
	case "7": // покинул огневой рубеж
		fmt.Fprintf(w, "%s The competitor(%s) left the firing range\n", line[0], line[2])
	case "8": // вошёл в штрафной круг
		biatlonist := biatlonistMap[line[2]]
		addPenaltyLap(biatlonist, line)
		fmt.Fprintf(w, "%s The competitor(%s) entered the penalty laps\n", line[0], line[2])
	case "9": // вышел из штрафного круга
		biatlonist := biatlonistMap[line[2]]
		updateLastPenaltyLap(biatlon, biatlonist, line)
		fmt.Fprintf(w, "%s The competitor(%s) left the penalty laps\n", line[0], line[2])
	case "10": // закончил основной круг
		fmt.Fprintf(w, "%s The competitor(%s) ended the main lap\n", line[0], line[2])
		biatlonist := biatlonistMap[line[2]]
		addMainLap(biatlon, biatlonist, line)
		biatlonist.TotalHits = len(biatlonist.LapsList) * 5 * biatlon.FiringLines
		if checkFinished(biatlon, biatlonist, line) {
			fmt.Fprintf(w, "%s The competitor(%s) has finished\n", line[0], line[2])
		}
	case "11": // не может продолжать
		biatlonist := biatlonistMap[line[2]]
		biatlonist.HeCantContinue = true
		fmt.Fprintf(w, "%s The competitor(%s) can`t continue: %s\n", line[0], line[2], strings.Join(line[3:], " "))
	}
}

func formattedTime(t time.Duration) string {
	hours := int(t.Hours())
	minutes := int(t.Minutes()) % 60
	seconds := int(t.Seconds()) % 60
	milliseconds := int(t.Milliseconds()) % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, milliseconds)
}

func CreateBiatlonist(line []string) *Biatlonist {
	biatlonist := new(Biatlonist)
	biatlonist.ID = line[2]
	biatlonist.HeIsDisqualified = false
	biatlonist.HeCantContinue = false
	biatlonist.LapsList = make([]LapData, 0)
	biatlonist.PenaltyList = make([]PenaltyData, 0)
	return biatlonist
}

func checkStartBiatlonist(biatlon *Biatlon, biatlonist *Biatlonist, line []string) error {
	biatlonist.StartTime = line[0][1 : len(line[0])-1]
	t1, _ := time.Parse("15:04:05.000", biatlonist.DrawingLots)
	t2, _ := time.Parse("15:04:05.000", biatlonist.StartTime)
	if t2.Before(t1) {
		biatlonist.HeIsDisqualified = true
		return errors.New("The competitor(%s) is disqualified")
	} else {
		diff := t2.Sub(t1)
		delta, _ := time.Parse("15:04:05", biatlon.StartDelta)
		zero := time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC)
		startDelta := delta.Sub(zero)
		if diff > startDelta {
			biatlonist.HeIsDisqualified = true
			return errors.New("The competitor(%s) is disqualified")
		} else {
			return nil
		}
	}
}

func addPenaltyLap(biatlonist *Biatlonist, line []string) {
	biatlonist.PenaltyList = append(biatlonist.PenaltyList, PenaltyData{
		TimeIn:  line[0][1 : len(line[0])-1],
		TimeOut: "",
		Speed:   0,
	})
}

func updateLastPenaltyLap(biatlon *Biatlon, biatlonist *Biatlonist, line []string) {
	bPen := &biatlonist.PenaltyList[len(biatlonist.PenaltyList)-1]
	bPen.TimeOut = line[0][1 : len(line[0])-1]
	t1, _ := time.Parse("15:04:05.000", bPen.TimeIn)
	t2, _ := time.Parse("15:04:05.000", bPen.TimeOut)
	diff := t2.Sub(t1)

	bPen.TotalTime = formattedTime(diff)

	bPen.Speed = float64(biatlon.PenaltyLen*biatlonist.LastCountMiss) / diff.Seconds()
}

func addMainLap(biatlon *Biatlon, biatlonist *Biatlonist, line []string) {
	if biatlonist.StartTime != "" && len(biatlonist.LapsList) == 0 {
		t1, _ := time.Parse("15:04:05.000", biatlonist.DrawingLots)
		t2, _ := time.Parse("15:04:05.000", line[0][1:len(line[0])-1])
		diff := t2.Sub(t1)
		biatlonist.LapsList = append(biatlonist.LapsList, LapData{
			TotalTime: formattedTime(diff),
			TimeOut:   line[0][1 : len(line[0])-1],
			Speed:     float64(biatlon.LapLen) / diff.Seconds(),
		})
	} else if biatlonist.StartTime != "" && len(biatlonist.LapsList) > 0 {
		t1, _ := time.Parse("15:04:05.000", biatlonist.LapsList[len(biatlonist.LapsList)-1].TimeOut)
		t2, _ := time.Parse("15:04:05.000", line[0][1:len(line[0])-1])
		diff := t2.Sub(t1)
		biatlonist.LapsList = append(biatlonist.LapsList, LapData{
			TotalTime: formattedTime(diff),
			TimeOut:   line[0][1 : len(line[0])-1],
			Speed:     float64(biatlon.LapLen) / diff.Seconds(),
		})
	}
}

func checkFinished(biatlon *Biatlon, biatlonist *Biatlonist, line []string) bool {
	if len(biatlonist.LapsList) == biatlon.Laps && !biatlonist.HeIsDisqualified && !biatlonist.HeCantContinue {
		t1, _ := time.Parse("15:04:05.000", biatlonist.DrawingLots)
		t2, _ := time.Parse("15:04:05.000", line[0][1:len(line[0])-1])
		diff := t2.Sub(t1)
		biatlonist.TotalTime = formattedTime(diff)
		return true
	}
	return false
}

func getConfig() (*Biatlon, error) {
	file, err := os.Open("config.json")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	jsonParser := json.NewDecoder(file)
	biatlon := new(Biatlon)
	err = jsonParser.Decode(biatlon)
	return biatlon, err
}
