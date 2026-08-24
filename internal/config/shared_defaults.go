package config

import "gopkg.in/yaml.v3"

func wellKnownServices() map[string]SharedServiceDef {
	scalar := func(s string) yaml.Node { return yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s} }
	cap16 := uint16(16)
	return map[string]SharedServiceDef{
		"postgres": {
			Image:       "postgres:16",
			Ports:       []string{"5432"},
			Command:     "postgres -c shared_preload_libraries=pg_stat_statements",
			Environment: map[string]string{"POSTGRES_USER": "postgres", "POSTGRES_PASSWORD": "postgres", "POSTGRES_DB": "postgres"},
			Volumes:     []string{"shared_postgres:/var/lib/postgresql/data"},
			Healthcheck: &HealthCheck{Test: scalar("pg_isready -U postgres -h 127.0.0.1"), Interval: "5s", Timeout: "3s", Retries: 3},
			DBUser:      "postgres",
			DBPassword:  "postgres",
		},
		"redis": {
			Image:    "redis:7-alpine",
			Ports:    []string{"6379"},
			Command:  "redis-server --appendonly yes",
			Volumes:  []string{"shared_redis:/data"},
			Capacity: &cap16,
		},
		"minio": {
			Image:       "minio/minio",
			Command:     `server /data --console-address ":9001"`,
			Ports:       []string{"9000", "9001"},
			Environment: map[string]string{"MINIO_ROOT_USER": "minioadmin", "MINIO_ROOT_PASSWORD": "minioadmin"},
			Volumes:     []string{"shared_minio:/data"},
			DBUser:      "minioadmin",
			DBPassword:  "minioadmin",
		},
		"opensearch": {
			Image:       "opensearchproject/opensearch:2.11.0",
			Ports:       []string{"9200", "9600"},
			Environment: map[string]string{"discovery.type": "single-node", "DISABLE_SECURITY_PLUGIN": "true", "OPENSEARCH_JAVA_OPTS": "-Xms256m -Xmx256m"},
			Volumes:     []string{"shared_opensearch:/usr/share/opensearch/data"},
		},
		"zincsearch": {
			Image:       "public.ecr.aws/zinclabs/zincsearch:latest",
			Ports:       []string{"4080"},
			Environment: map[string]string{"ZINC_FIRST_ADMIN_USER": "admin", "ZINC_FIRST_ADMIN_PASSWORD": "admin", "ZINC_DATA_PATH": "/data"},
			Volumes:     []string{"shared_zincsearch:/data"},
			DBUser:      "admin",
			DBPassword:  "admin",
		},
	}
}

func applyWellKnownDefaults(cfg *Config) {
	if cfg == nil || len(cfg.SharedServices) == 0 {
		return
	}
	tmpls := wellKnownServices()
	for name, def := range cfg.SharedServices {
		if def == nil {
			def = &SharedServiceDef{}
			cfg.SharedServices[name] = def
		}
		kind := def.Type
		if kind == "" {
			kind = name
		}
		if t, ok := tmpls[kind]; ok {
			fillSharedDefaults(def, t)
		}
	}
}

func fillSharedDefaults(d *SharedServiceDef, t SharedServiceDef) {
	if d.Image == "" {
		d.Image = t.Image
	}
	if len(d.Ports) == 0 {
		d.Ports = t.Ports
	}
	if d.Command == "" {
		d.Command = t.Command
	}
	if len(d.Volumes) == 0 {
		d.Volumes = t.Volumes
	}
	if d.Healthcheck == nil {
		d.Healthcheck = t.Healthcheck
	}
	if d.DBUser == "" {
		d.DBUser = t.DBUser
	}
	if d.DBPassword == "" {
		d.DBPassword = t.DBPassword
	}
	if d.Capacity == nil {
		d.Capacity = t.Capacity
	}
	if len(t.Environment) > 0 {
		merged := make(map[string]string, len(t.Environment)+len(d.Environment))
		for k, v := range t.Environment {
			merged[k] = v
		}
		for k, v := range d.Environment {
			merged[k] = v
		}
		d.Environment = merged
	}
}
