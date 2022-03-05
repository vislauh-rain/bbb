package bbb

import (
	"math/rand"
	"time"
)

type urlContext struct {
	Now      time.Time
	RandDate time.Time
}

func (c urlContext) RandStr() string {
	return randStringRunes(3 + randGen.Intn(7))
}

func (c urlContext) RandUInt() int32 {
	return randGen.Int31()
}

var (
	letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	randGen     *rand.Rand
)

func init() {
	randGen = rand.New(rand.NewSource(time.Now().UnixNano()))
}

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[randGen.Intn(len(letterRunes))]
	}
	return string(b)
}

var randTimeStart = time.Date(2019, time.January, 1, 0, 0, 0, 0, time.UTC)

func randTime() time.Time {
	days := (int(time.Now().Sub(randTimeStart).Hours()) / 24) + 1
	return randTimeStart.AddDate(0, 0, randGen.Intn(days))
}
