package config

var CoreConfig baseConfig

type baseConfig struct {
	Port     int          `yaml:"port,omitempty"`
	SchemaDB string       `yaml:"schemaDB,omitempty"`
	Postgres GormProperty `yaml:"postgres,omitempty"`
}

type GormProperty struct {
	Host     string `yaml:"host,omitempty"`
	Port     int    `yaml:"port,omitempty"`
	Database string `yaml:"database,omitempty"`
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
	DSN      string `yaml:"dsn,omitempty"`
	ShowSql  bool   `yaml:"showSql,omitempty"`
}
