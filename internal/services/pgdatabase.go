package services

import (
	"fmt"
	"os/exec"
	"strings"
)

func psqlExec(host string, port uint16, user, pass, db, sql string) (string, error) {
	if db == "" {
		db = "postgres"
	}
	if container := findPostgresContainer(port); container != "" {
		out, err := exec.Command("docker", "exec", container, "psql", "-U", user, "-d", db, "-tAc", sql).CombinedOutput()
		return string(out), err
	}
	extraHost := fmt.Sprintf("--add-host=%s:host-gateway", host)
	connURL := fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", user, pass, host, port, db)
	out, err := exec.Command("docker", "run", "--rm", extraHost, "postgres:16-alpine", "psql", connURL, "-tAc", sql).CombinedOutput()
	return string(out), err
}

func DatabaseExists(host string, port uint16, user, pass, db string) bool {
	out, err := psqlExec(host, port, user, pass, "", fmt.Sprintf("SELECT 1 FROM pg_database WHERE datname='%s'", db))
	return err == nil && strings.TrimSpace(out) == "1"
}

func CreateDatabase(host string, port uint16, user, pass, db string) error {
	out, err := psqlExec(host, port, user, pass, "", fmt.Sprintf(`CREATE DATABASE "%s"`, db))
	if err != nil && !strings.Contains(out, "already exists") {
		return fmt.Errorf("create db %s: %s", db, strings.TrimSpace(out))
	}
	return nil
}

func DropDatabase(host string, port uint16, user, pass, db string) error {
	terminate(host, port, user, pass, db)
	out, err := psqlExec(host, port, user, pass, "", fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, db))
	if err != nil {
		return fmt.Errorf("drop db %s: %s", db, strings.TrimSpace(out))
	}
	return nil
}

func terminate(host string, port uint16, user, pass, db string) {
	_, _ = psqlExec(host, port, user, pass, "",
		fmt.Sprintf("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='%s' AND pid<>pg_backend_pid()", db))
}

func CloneDatabase(host string, port uint16, user, pass, template, target string) error {
	if !DatabaseExists(host, port, user, pass, template) {
		return fmt.Errorf("template db %q does not exist", template)
	}
	terminate(host, port, user, pass, target)
	if err := DropDatabase(host, port, user, pass, target); err != nil {
		return err
	}
	terminate(host, port, user, pass, template)
	out, err := psqlExec(host, port, user, pass, "",
		fmt.Sprintf(`CREATE DATABASE "%s" TEMPLATE "%s"`, target, template))
	if err != nil {
		return fmt.Errorf("clone %s from %s: %s", target, template, strings.TrimSpace(out))
	}
	return nil
}
