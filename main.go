/*

Copyright 2023 Fancy Bits, LLC

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the “Software”), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED “AS IS”, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

*/

package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

var (
	tunerLock sync.Mutex
	tuners    []tuner
	envdebug  bool = true
)

type tuner struct {
	url     string
	pre     string
	start   string
	stop    string
	tunerip string
	reboot  string
	active  bool
}

type reader struct {
	io.ReadCloser
	t       *tuner
	channel string
	started bool
}

func init() {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 5 * time.Second
	transport.DialContext = (&net.Dialer{
		Timeout: 5 * time.Second,
	}).DialContext
	http.DefaultClient.Transport = transport
}

func (r *reader) Read(p []byte) (int, error) {
	if !r.started {
		r.started = true
		go func() {
			if err := execute(r.t.start, r.channel, r.t.tunerip); err != nil {
				log.Printf("[ERR] Failed to run start script: %v", err)
				return
			}
		}()
	}
	return r.ReadCloser.Read(p)
}

func (r *reader) Close() error {
	if err := execute(r.t.stop, r.t.tunerip); err != nil {
		log.Printf("[ERR] Failed to run stop script: %v", err)
		execute(r.t.reboot, r.t.tunerip)
	}
	tunerLock.Lock()
	r.t.active = false
	tunerLock.Unlock()
	return r.ReadCloser.Close()
}

func execute(args ...string) error {
	t0 := time.Now()
	log.Printf("Running %v", args)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	log.Printf("Finished running %v in %v", args[0], time.Since(t0))
	return err
}

func tune(idx, channel string) (io.ReadCloser, error) {
	tunerLock.Lock()
	defer tunerLock.Unlock()
	var t *tuner
	if idx == "" || idx == "auto" {
		for i, ti := range tuners {
			if ti.active {
				continue
			}
			t = &tuners[i]
			log.Printf("Attempting tune for device %s %v %v", t.tunerip, channel, idx)
			resp, err := http.Get(t.url)
			if err != nil {
				log.Printf("[ERR] Failed to fetch source: %v", err)
				continue
			} else if resp.StatusCode != 200 {
				log.Printf("[ERR] Failed to fetch source: %v", resp.Status)
				continue
			}
			if err := execute(t.pre, t.tunerip); err != nil {
				log.Printf("[ERR] Failed to run pre script: %v %s", err, t.tunerip)
				continue
			}
			t.active = true
			return &reader{
				ReadCloser: resp.Body,
				channel:    channel,
				t:          t,
			}, nil
		}
	} else {
		i, _ := strconv.Atoi(idx)
		if i < len(tuners) && i >= 0 {
			t = &tuners[i]
			log.Printf("Attempting tune for device %s", t.tunerip)
			resp, err := http.Get(t.url)
			if err != nil {
				log.Printf("[ERR] Failed to fetch source: %v", err)
				return nil, err
			} else if resp.StatusCode != 200 {
				log.Printf("[ERR] Failed to fetch source: %v", resp.Status)
				return nil, fmt.Errorf("invalid response: %v", resp.Status)
			}
			if err := execute(t.pre, t.tunerip); err != nil {
				log.Printf("[ERR] Failed to run pre script: %v", err)
				return nil, fmt.Errorf("tuner '%s' not available", t.tunerip)
			}
			t.active = true
			return &reader{
				ReadCloser: resp.Body,
				channel:    channel,
				t:          t,
			}, nil

		}
	}
	return nil, fmt.Errorf("tuners not available")
}

func run() error {
	r := gin.Default()
	r.SetTrustedProxies(nil)
	r.GET("/play/tuner:tuner/:channel", func(c *gin.Context) {
		tuner := c.Param("tuner")
		channel := c.Param("channel")
		c.Header("Transfer-Encoding", "identity")
		c.Header("Content-Type", "video/mp2t")
		c.Writer.WriteHeaderNow()
		c.Writer.Flush()
		reader, err := tune(tuner, channel)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		defer func() {
			reader.Close()
		}()
		io.Copy(c.Writer, reader)
	})
	return r.Run(":7654")
}

func main() {
	// Load environment variables from env if the file exists.
	if _, errenv := os.Stat("env"); errenv == nil {
		if envdebug == true {
			log.Printf("[ENV] Loading env")
		}
		godotenv.Load("env")
	}
	// Get the number of tuners from the environment variable
	numTunersStr := os.Getenv("NUMBER_TUNERS")
	numTuners, errtuners := strconv.Atoi(numTunersStr)
	if errtuners != nil {
		panic("Could not find an environment variabled named NUMBER_TUNERS")
	}
	// Get directory of scripts
	streamerApp := os.Getenv("STREAMER_APP")
	// Loop over the number of tuners and create each one
	for i := 1; i <= numTuners; i++ {
		iStr := strconv.Itoa(i)
		var tunerurl string = "TUNER" + iStr + "_URL"
		var tunerip string = "TUNER" + iStr + "_IP"
		t := tuner{
			url:     os.Getenv(tunerurl),
			pre:     "./" + streamerApp + "/prebmitune.sh",
			start:   "./" + streamerApp + "/bmitune.sh",
			stop:    "./" + streamerApp + "/stopbmitune.sh",
			reboot:  "./" + streamerApp + "/reboot.sh",
			tunerip: os.Getenv(tunerip),
		}
		if envdebug == true {
			log.Printf("[ENV] Creating tuner %d", i)
			log.Printf("[ENV] TUNER%s_URL     %s", iStr, os.Getenv(tunerurl))
			log.Printf("[ENV] TUNER%s_IP      %s", iStr, os.Getenv(tunerip))
			log.Printf("[ENV] PRE SCRIPT     %s", "./"+streamerApp+"/prebmitune.sh")
			log.Printf("[ENV] START SCRIPT   %s", "./"+streamerApp+"/bmitune.sh")
			log.Printf("[ENV] STOP SCRIPT    %s", "./"+streamerApp+"/stopbmitune.sh")
			log.Printf("[ENV] REBOOT SCRIPT  %s", "./"+streamerApp+"/reboot.sh")
			log.Printf("\n")
		}
		// Add the tuner to the tuners slice
		tuners = append(tuners, t)
	}
	// Start GIN
	errrun := run()
	if errrun != nil {
		panic(errrun)
	}
}
