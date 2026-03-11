package main

import (
	"embed"

	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/joho/godotenv"
)

// TODO: Implemeng flags in the future

// CertificateRequest represents a single certificate request read from the CSV file.
// It contains the:
// domain for which the certificate is requested,
// the DNS profile to use as dns provider definition,
// the email address associated with the request.
type CertificateRequest struct {
	Domain     string
	DNSProfile string
	Email      string
}

// Default DNS Profile Configuration Path
const DNSProfileConfigPath = "./dns_profiles"

// main is the entry point of the application. It performs the following steps:
// 1. Checks if the DNS profile configuration directory exists at the specified path.
//   - If it does not exist, it creates example DNS profile environment files from embedded templates.
//   - If it exists but is not a directory, it logs a fatal error and exits.
//   - If it exists and is a directory, it logs that the directory already exists and skips file creation.
//
// 2. Reads the certificates.csv file and returns a slice of CertificateRequest structs.
//   - If there is an error reading the CSV file, it logs a fatal error and exits.
//
// 3. Iterates over each CertificateRequest in the slice:
//   - Loads environment variables for each unique DNS profile used in the certs slice before executing lego commands.
//   - If there is an error loading the DNS profile environment variables, it logs the error and skips the certificate request for that domain.
//   - Executes the lego command for each certificate request, passing the appropriate arguments for domain, dns profile, and email.
//   - If there is an error executing the lego command, it logs the error and continues to the next certificate request.
func main() {
	// Check if the DNS profile configuration directory exists at the specified path
	info, err := os.Stat(DNSProfileConfigPath)
	switch {
	case os.IsNotExist(err):
		log.Printf(
			"[ WARN ] %s does not exist; it is required to load DNS profile environment variables for lego execution.\n",
			DNSProfileConfigPath,
		)
		log.Printf(
			"[ INFO ] %s does not exist; creating example DNS profile environment files.\nSee example *.env files in the ./dns_profiles directory for reference.",
			DNSProfileConfigPath,
		)

		if err := CreateExampleDNSProfileEnvFiles(); err != nil {
			log.Fatalf("[ WARN ] failed to create example DNS profile environment files: %v", err)
		}

	case err != nil:
		log.Fatalf("[ WARN ] failed to check %s: %v", DNSProfileConfigPath, err)

	case !info.IsDir():
		log.Fatalf("path %s exists but is not a directory", DNSProfileConfigPath)

	default:
		log.Printf(
			"[ INFO ] %s directory already exists; creating example DNS profile environment files.\nSee example *-example.env files in the ./dns_profiles directory for reference.",
			DNSProfileConfigPath,
		)
	}

	// Read CSV file and return a slice of CertificateRequest structs read from the certificates.csv file
	certs, err := ReadCertificatesCSV("certificates.csv")
	if err != nil {
		log.Fatalf("[ WARN ] failed to read certificates.csv: %v", err)
	}

	//
	for _, cert := range certs {
		// Load environment variables for each unique DNS profile used in the certs slice before executing lego commands
		if err := LoadDNSProfileEnv(cert.DNSProfile, DNSProfileConfigPath); err != nil {
			log.Printf("[ WARN ] failed to load DNS profile environment variables for profile=%s: %v", cert.DNSProfile, err)
			log.Printf("skipping certificate request for domain=%s due to DNS profile loading error\n", cert.Domain)
			continue
		}

		// Execute lego command for each certificate request in the certs slice, passing the appropriate arguments for domain, dns profile, and email
		if err := RunLego(cert); err != nil {
			log.Printf(
				"[ WARN] lego execution failed for domain=%s dns_profile=%s email=%s: %v",
				cert.Domain,
				cert.DNSProfile,
				cert.Email,
				err,
			)
			continue
		}
	}
}

// ReadCertificatesCSV reads the CSV file at the given path and returns a slice of CertificateRequest.
func ReadCertificatesCSV(path string) ([]CertificateRequest, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true // trim leading spaces from fields

	var results []CertificateRequest
	lineNumber := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read csv line %d: %w", lineNumber+1, err)
		}
		// Output for each line read from the CSV file
		log.Printf("[ INFO ] read line %d: %v\n", lineNumber+1, record)

		lineNumber++

		// Skip empty lines
		if len(record) == 0 {
			continue
		}

		// Expect exactly: domains/fqdn,dns_profile,email
		if len(record) < 3 {
			return nil, fmt.Errorf(
				"line %d: expected 3 columns (domain/fqdn,dns_profile,email), got %d",
				lineNumber,
				len(record),
			)
		}

		domain := strings.TrimSpace(record[0])
		DNSProfile := strings.TrimSpace(record[1])
		email := strings.TrimSpace(record[2])

		// Skip header row
		if lineNumber == 1 &&
			strings.EqualFold(domain, "domain/fqdn") &&
			strings.EqualFold(DNSProfile, "dns_profile") &&
			strings.EqualFold(email, "email") {
			continue
		}

		if domain == "" || DNSProfile == "" || email == "" {
			return nil, fmt.Errorf(
				"line %d: domain, dns_profile, and email must not be empty",
				lineNumber,
			)
		}

		results = append(results, CertificateRequest{
			Domain:     domain,
			DNSProfile: DNSProfile,
			Email:      email,
		})
	}

	return results, nil
}

// RunLego validates the lego binary and prepares the lego command for the given CertificateRequest.
func RunLego(cert CertificateRequest) error {
	file, err := exec.LookPath("lego")
	if err != nil {
		log.Printf("[ ERROR ] lego binary not found in PATH: %v", err)
		fmt.Println("Please install lego and ensure it is in your system's PATH.")
		return fmt.Errorf("lego binary not found in PATH: %w", err)
	}

	log.Printf("[ INFO ] found lego binary at: %s\n", file)

	args := []string{
		"--dns", cert.DNSProfile,
		"--email", cert.Email,
		"--domains", cert.Domain,
		"--server", "https://acme-v02.api.letsencrypt.org/directory",
		// "--path", "/path/to/certs",
		// "run",
	}

	log.Printf("[ DEBUG ] CMD Execution | %s %s\n", file, strings.Join(args, " "))

	// TODO: Enable command execution once request validation and output path handling are finalized.
	/*
		cmd := exec.Command(file, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("run lego: %w", err)
		}
	*/

	return nil
}

// LoadDNSProfileEnv loads the environment variables from the specified DNS profile .env file.
func LoadDNSProfileEnv(dns_profile, dns_profile_path string) error {
	// Check if ./dns_profiles/{dns_profile}.env file exists
	envFile := fmt.Sprintf("%s/%s.env", dns_profile_path, dns_profile)
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		return fmt.Errorf("DNS profile environment file not found: %s", envFile)
	} else {
		log.Printf("[ INFO ] loading environment variables from %s\n", envFile)
	}

	// Load the environment variables from the file
	if err := godotenv.Load(envFile); err != nil {
		return fmt.Errorf("[ WARN ] failed to load DNS profile environment variables: %w", err)
	}

	return nil
}

// Embed the DNS profile .env templates into the binary using go:embed
//
//go:embed templates/*.env
var profileTemplates embed.FS

// CreateExampleDNSProfileEnvFiles creates the ./dns_profiles directory
// if it does not exist and writes example DNS profile .env files
// from the embedded templates.
func CreateExampleDNSProfileEnvFiles() error {
	if err := os.MkdirAll(DNSProfileConfigPath, 0o755); err != nil {
		return fmt.Errorf("[ WARN ] failed to create dns_profiles directory: %w", err)
	}

	entries, err := profileTemplates.ReadDir("templates")
	if err != nil {
		return fmt.Errorf("[ WARN ] failed to read embedded templates directory: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		templatePath := "templates/" + e.Name()
		outputPath := fmt.Sprintf("%s/%s", DNSProfileConfigPath, e.Name())

		if _, err := os.Stat(outputPath); err == nil {
			log.Printf("[ INFO ] skipping existing file %s\n", outputPath)
			continue
		}

		data, err := profileTemplates.ReadFile(templatePath)
		if err != nil {
			return fmt.Errorf("[ WARN ] failed to read embedded template %s: %w", templatePath, err)
		}

		if err := os.WriteFile(outputPath, data, 0o600); err != nil {
			return fmt.Errorf("[ WARN ] failed to write example profile %s: %w", outputPath, err)
		}

		log.Printf("[ INFO ] Example DNS profile environment file created: %s\n", outputPath)
	}

	return nil
}
