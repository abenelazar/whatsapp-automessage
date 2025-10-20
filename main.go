package main

import (
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

type contact struct {
	Data  map[string]string
	Order []string
}

func main() {
	rand.Seed(time.Now().UnixNano())

	contactsPath := flag.String("contacts", "contacts.csv", "path to contacts csv")
	messagePath := flag.String("message", "message.txt", "path to message template")
	completedPath := flag.String("completed", "completed.csv", "path to completed csv")
	sessionDir := flag.String("session", "whatsapp-session", "directory for persistent browser session")
	minDelay := flag.Duration("min-delay", 3*time.Second, "minimum delay between messages")
	maxDelay := flag.Duration("max-delay", 7*time.Second, "maximum delay between messages")
	headless := flag.Bool("headless", false, "run Chrome in headless mode")
	flag.Parse()

	if *maxDelay < *minDelay {
		log.Fatalf("max-delay must be >= min-delay")
	}

	contacts, err := readContacts(*contactsPath)
	if err != nil {
		log.Fatalf("failed to read contacts: %v", err)
	}
	if len(contacts) == 0 {
		log.Println("no contacts found - nothing to do")
		return
	}

	templateText, err := os.ReadFile(*messagePath)
	if err != nil {
		log.Fatalf("failed to read message template: %v", err)
	}

	completed, err := readCompleted(*completedPath)
	if err != nil {
		log.Fatalf("failed to read completed csv: %v", err)
	}

	if err := os.MkdirAll(*sessionDir, 0o755); err != nil {
		log.Fatalf("failed to create session directory: %v", err)
	}

	ctx, cancel := createBrowserContext(*sessionDir, *headless)
	defer cancel()

	for idx, c := range contacts {
		number := strings.TrimSpace(c.Data["number"])
		if number == "" {
			log.Printf("[%d] skipping contact without number: %+v", idx+1, c.Data)
			continue
		}

		if completed[number] {
			log.Printf("[%d] already completed for %s, skipping", idx+1, number)
			continue
		}

		message, err := renderTemplate(string(templateText), c.Data)
		if err != nil {
			log.Printf("[%d] failed to render template for %s: %v", idx+1, number, err)
			continue
		}

		if err := sendMessage(ctx, number, message); err != nil {
			if errors.Is(err, errInvalidNumber) {
				log.Printf("[%d] invalid number %s, skipping", idx+1, number)
				continue
			}
			log.Printf("[%d] failed to send to %s: %v", idx+1, number, err)
			continue
		}

		if err := appendCompleted(*completedPath, c); err != nil {
			log.Printf("[%d] failed to append completed for %s: %v", idx+1, number, err)
		} else {
			completed[number] = true
			log.Printf("[%d] message sent to %s", idx+1, number)
		}

		delay := randomDelay(*minDelay, *maxDelay)
		log.Printf("waiting %s before next contact", delay)
		time.Sleep(delay)
	}
}

func readContacts(path string) ([]contact, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	headers, err := reader.Read()
	if err != nil {
		return nil, err
	}

	var contacts []contact
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		data := map[string]string{}
		for i, header := range headers {
			if i < len(row) {
				data[header] = row[i]
			} else {
				data[header] = ""
			}
		}
		contacts = append(contacts, contact{Data: data, Order: append([]string(nil), headers...)})
	}
	return contacts, nil
}

func readCompleted(path string) (map[string]bool, error) {
	completed := map[string]bool{}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return completed, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	headers, err := reader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return completed, nil
		}
		return nil, err
	}

	index := -1
	for i, h := range headers {
		if strings.EqualFold(h, "number") {
			index = i
			break
		}
	}
	if index == -1 {
		return completed, nil
	}

	for {
		row, err := reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if index < len(row) {
			completed[strings.TrimSpace(row[index])] = true
		}
	}
	return completed, nil
}

func appendCompleted(path string, c contact) error {
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		file, err := os.Create(path)
		if err != nil {
			return err
		}
		defer file.Close()

		writer := csv.NewWriter(file)

		headers := append([]string(nil), c.Order...)
		if len(headers) == 0 {
			for header := range c.Data {
				headers = append(headers, header)
			}
			sort.Strings(headers)
		}

		if err := writer.Write(headers); err != nil {
			return err
		}
		record := make([]string, len(headers))
		for i, header := range headers {
			record[i] = c.Data[header]
		}
		if err := writer.Write(record); err != nil {
			return err
		}
		writer.Flush()
		return writer.Error()
	}

	if err != nil {
		return err
	}

	headers, err := readHeader(path)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)

	record := make([]string, len(headers))
	for i, header := range headers {
		record[i] = c.Data[header]
	}
	if err := writer.Write(record); err != nil {
		return err
	}
	writer.Flush()
	return writer.Error()
}

func readHeader(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	headers, err := reader.Read()
	if err != nil {
		return nil, err
	}
	return headers, nil
}

func renderTemplate(templateText string, data map[string]string) (string, error) {
	tmpl, err := template.New("message").Parse(templateText)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	if err := tmpl.Execute(&sb, data); err != nil {
		return "", err
	}
	return sb.String(), nil
}

var errInvalidNumber = errors.New("invalid number")

func createBrowserContext(sessionDir string, headless bool) (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", headless),
		chromedp.Flag("disable-gpu", headless),
		chromedp.UserDataDir(filepath.Clean(sessionDir)),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancelCtx := chromedp.NewContext(allocCtx)

	cancel := func() {
		cancelCtx()
		cancelAlloc()
	}
	return ctx, cancel
}

func sendMessage(ctx context.Context, number, message string) error {
	chatURL := fmt.Sprintf("https://web.whatsapp.com/send?phone=%s&app_absent=0", number)
	if err := chromedp.Run(ctx, chromedp.Navigate(chatURL)); err != nil {
		return err
	}

	invalid, err := waitForChatOrInvalid(ctx)
	if err != nil {
		return err
	}
	if invalid {
		dismissInvalid(ctx)
		return errInvalidNumber
	}

	inputSelectors := []string{
		"div[contenteditable='true'][data-tab='10']",
		"div[contenteditable='true'][role='textbox']",
	}

	var inputSel string
	for _, sel := range inputSelectors {
		if err := chromedp.Run(ctx, chromedp.WaitVisible(sel, chromedp.ByQuery, chromedp.NodeVisible)); err == nil {
			inputSel = sel
			break
		}
	}
	if inputSel == "" {
		return errors.New("could not find message input field")
	}

	actions := []chromedp.Action{
		chromedp.Focus(inputSel, chromedp.ByQuery),
		chromedp.SendKeys(inputSel, message, chromedp.ByQuery),
		chromedp.SendKeys(inputSel, kb.Enter, chromedp.ByQuery),
	}

	if err := chromedp.Run(ctx, actions...); err != nil {
		return err
	}

	return nil
}

func waitForChatOrInvalid(ctx context.Context) (bool, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return false, timeoutCtx.Err()
		case <-ticker.C:
			var isInvalid bool
			if err := chromedp.Run(timeoutCtx, chromedp.Evaluate(`document.body.innerText.includes("Phone number shared via url is invalid")`, &isInvalid)); err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return false, err
				}
				continue
			}
			if isInvalid {
				return true, nil
			}

			var ready bool
			if err := chromedp.Run(timeoutCtx, chromedp.Evaluate(`!!document.querySelector("div[contenteditable='true'][data-tab='10']") || !!document.querySelector("div[contenteditable='true'][role='textbox']")`, &ready)); err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return false, err
				}
				continue
			}
			if ready {
				return false, nil
			}
		}
	}
}

func dismissInvalid(ctx context.Context) {
	selectors := []string{
		`div[role='button'][data-testid='popup-controls-ok']`,
		`div[role='button'][aria-label='OK']`,
		`div[role='button'][title='OK']`,
		`button[title='Close']`,
	}
	for _, sel := range selectors {
		if err := chromedp.Run(ctx, chromedp.Click(sel, chromedp.ByQuery, chromedp.NodeVisible)); err == nil {
			return
		}
	}
}

func randomDelay(min, max time.Duration) time.Duration {
	if max <= min {
		return min
	}
	diff := max - min
	return min + time.Duration(rand.Int63n(int64(diff)+1))
}
