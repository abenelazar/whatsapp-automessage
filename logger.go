package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

var (
	logLevel   string
	logFile    *os.File
	infoLogger *log.Logger
	warnLogger *log.Logger
	errLogger  *log.Logger
)

func InitLogger(config *Config) error {
	logLevel = strings.ToLower(config.Logging.Level)

	// Open log file if specified
	if config.Logging.OutputFile != "" {
		var err error
		logFile, err = os.OpenFile(config.Logging.OutputFile,
			os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
	}

	// Setup loggers
	flags := log.Ldate | log.Ltime
	if logFile != nil {
		infoLogger = log.New(logFile, "[INFO] ", flags)
		warnLogger = log.New(logFile, "[WARN] ", flags)
		errLogger = log.New(logFile, "[ERROR] ", flags)
	} else {
		infoLogger = log.New(os.Stdout, "[INFO] ", flags)
		warnLogger = log.New(os.Stdout, "[WARN] ", flags)
		errLogger = log.New(os.Stderr, "[ERROR] ", flags)
	}

	return nil
}

func CloseLogger() {
	if logFile != nil {
		logFile.Close()
	}
}

func Log(level, message string) {
	level = strings.ToLower(level)

	// Determine if we should log based on configured level
	levelPriority := map[string]int{
		"debug": 0,
		"info":  1,
		"warn":  2,
		"error": 3,
	}

	configuredPriority := levelPriority[logLevel]
	messagePriority := levelPriority[level]

	if messagePriority < configuredPriority {
		return // Skip logging
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	formattedMsg := fmt.Sprintf("[%s] %s", timestamp, message)

	switch level {
	case "debug", "info":
		if infoLogger != nil {
			infoLogger.Println(message)
		}
		fmt.Println(formattedMsg)
	case "warn":
		if warnLogger != nil {
			warnLogger.Println(message)
		}
		fmt.Println(formattedMsg)
	case "error":
		if errLogger != nil {
			errLogger.Println(message)
		}
		fmt.Fprintln(os.Stderr, formattedMsg)
	}
}

func Logf(level, format string, args ...interface{}) {
	Log(level, fmt.Sprintf(format, args...))
}
