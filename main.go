package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/kwkoo/configparser"
	nut "github.com/robbiet480/go.nut"
)

var client nut.Client
var connectClient = true
var logFile *os.File

func main() {
	config := struct {
		Server                   string `env:"MONITOR" flag:"monitor" usage:"should be in the format ups@system or just system (where system is the hostname or IP address of the NUT server) - if the UPS name is omitted the client will query for all UPS's and will use the first UPS returned, which is more inefficient" mandatory:"true"`
		Command                  string `env:"SHUTDOWNCMD" flag:"shutdowncmd" default:"/sbin/poweroff" usage:"command to execute once the UPS is determined to be on battery power"`
		Sleep                    int    `env:"POLLFREQ" flag:"pollfreq" default:"10" usage:"number of seconds between status checks - note that the NUT server will terminate connections after 60 seconds"`
		HealthyMessageIterations int    `env:"HEALTHITERATIONS" flag:"healthiterations" default:"360" usage:"the rate at which healthy messages are logged - if this is set to 100 then a healthy message will be logged once every 100 iterations"`
		LogFilename              string `env:"LOG" flag:"log" usage:"log filename - logs to stdout if omitted"`
	}{}
	if err := configparser.Parse(&config); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing configuration: %v\n", err)
		os.Exit(1)
	}

	upsName, server := extractUpsNameFromMonitorOption(config.Server)

	if len(config.LogFilename) > 0 {
		var err error
		logFile, err = os.OpenFile(config.LogFilename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			flag.PrintDefaults()
			fmt.Fprintf(os.Stderr, "could not open log file %s: %v\n", config.LogFilename, err)
			os.Exit(1)
		}
		defer logFile.Close()
		log.SetOutput(logFile)
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP)

	log.Println("NUT server:", server)

	tmpName := upsName
	if len(tmpName) == 0 {
		tmpName = "UPS name not specified - will query for all UPS's"
	}
	log.Println("UPS name:", tmpName)

	log.Println("polling frequency:", config.Sleep, "seconds")
	log.Println("health iterations:", config.HealthyMessageIterations, "iterations")
	log.Println("shutdown command:", config.Command)
	if len(config.LogFilename) > 0 {
		log.Println("log file:", config.LogFilename)
	} else {
		log.Println("logging to stdout")
	}

	// log a healthy message the first time around
	healthyCount := config.HealthyMessageIterations

	for {
		status, err := getUpsStatus(server, upsName)
		if err != nil {
			log.Println("could not get UPS status:", err)
			healthyCount = config.HealthyMessageIterations
		} else {
			if strings.HasPrefix(status, "OB") {
				disconnectClient()
				log.Printf("detected unhealthy UPS status %s, initiating command %s\n", status, config.Command)
				cmdSlice := strings.Split(config.Command, " ")
				cmd := exec.Command(cmdSlice[0], cmdSlice[1:]...)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				err := cmd.Run()
				if err != nil {
					log.Fatalln("command finished with error:", err)
				}
				log.Println("done executing command, exiting...")
				os.Exit(0)
			}

			healthyCount++
			if healthyCount >= config.HealthyMessageIterations {
				healthyCount = 0
				log.Println("detected healthy UPS status", status)
			}
		}

		select {
		case <-sig:
			log.Println("SIGHUP received")
			healthyCount = config.HealthyMessageIterations
			reopenLogFile(config.LogFilename)
		case <-time.After(time.Second * time.Duration(config.Sleep)):

		}
	}
}

func reopenLogFile(logFilename string) {
	if logFile == nil {
		return
	}

	log.Println("closing log file...")
	logFile.Close()

	logFile, err := os.OpenFile(logFilename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logFile = nil
		log.SetOutput(os.Stdout)
		log.Printf("could not reopen log file %s: %v", logFilename, err)
		return
	}
	log.SetOutput(logFile)
	log.Println("successfully reopened log file", logFilename)
}

func extractUpsNameFromMonitorOption(server string) (string, string) {
	at := strings.Index(server, "@")
	if at == -1 {
		return "", server
	}

	return server[:at], server[at+1:]
}

func getUpsStatusForUps(upsName string) (string, error) {
	resp, err := client.SendCommand(fmt.Sprintf("GET VAR %s ups.status", upsName))
	if err != nil {
		return "", fmt.Errorf("error retrieving variable: %v", err)
	}
	for _, line := range resp {
		if strings.HasPrefix(line, "VAR ") && strings.Contains(line, " ups.status ") {
			left := strings.Index(line, "\"")
			right := strings.LastIndex(line, "\"")

			if left == -1 || right == -1 || left == right {
				continue
			}

			return line[left+1 : right], nil
		}
	}

	return "", fmt.Errorf("could not find ups.status in response")
}

func getUpsStatus(server, upsName string) (string, error) {
	var err error
	if connectClient {
		client, err = nut.Connect(server)
		if err != nil {
			return "", fmt.Errorf("could not connect to NUT server: %v", err)
		}
		connectClient = false
	}

	if len(upsName) > 0 {
		status, err := getUpsStatusForUps(upsName)
		if err == nil {
			return status, nil
		}

		disconnectClient()
		return "", fmt.Errorf("could not retrieve ups.status variable from UPS %s: %v", upsName, err)
	}

	// the user has not specified a ups name, so we'll get everything

	upsList, err := client.GetUPSList()
	if err != nil {
		if isEOF(err) {
			return "", err
		}
		disconnectClient()
		return "", fmt.Errorf("error retrieving UPS list: %v", err)
	}

	if len(upsList) < 1 {
		// we don't exit because the NUT server could just have been started
		// and it hasn't sensed the UPS cable yet
		disconnectClient()
		return "", fmt.Errorf("retrieved empty UPS list")
	}

	v, err := getVariableWithName(upsList[0], "ups.status")
	if err != nil {
		disconnectClient()
		return "", fmt.Errorf("error getting variable ups.status from UPS %s: %v", upsList[0].Name, err)
	}

	value, ok := v.Value.(string)
	if !ok {
		disconnectClient()
		return "", fmt.Errorf("value of variable ups.status is not a string")
	}

	return value, nil
}

func getVariableWithName(ups nut.UPS, name string) (nut.Variable, error) {
	vars, err := ups.GetVariables()
	if err != nil {
		return nut.Variable{}, fmt.Errorf("could not get variable %s: %v", name, err)
	}

	for _, v := range vars {
		if name == v.Name {
			return v, nil
		}
	}
	return nut.Variable{}, fmt.Errorf("variable %s does not exist", name)
}

// If error was an EOF, set to connect to server the next round and return true.
func isEOF(err error) bool {
	if strings.HasSuffix(err.Error(), " EOF") {
		connectClient = true
		return true
	}
	return false
}

func disconnectClient() {
	log.Println("disconnecting from server")
	client.Disconnect()
	connectClient = true
}
