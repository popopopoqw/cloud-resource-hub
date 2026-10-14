package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"resource-center/internal/pan123web"
)

func main() {
	env := map[string]string{}
	for _, l := range readFileLines(".env") {
		if i := strings.Index(l, "="); i > 0 && !strings.HasPrefix(l, "#") {
			env[strings.TrimSpace(l[:i])] = strings.TrimSpace(l[i+1:])
		}
	}
	tok := env["PAN123_TOKEN"]
	c := pan123web.New(tok, env["PAN123_ACCOUNT"], env["PAN123_PASSWORD"])
	start := time.Now()
	size, err := c.DirSize(32224194)
	fmt.Printf("动漫目录大小: %d (%.2fGB) 耗时: %v err: %v\n", size, float64(size)/1e9, time.Since(start), err)
}

func readFileLines(p string) []string {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	return strings.Split(string(b), "\n")
}
