package postgres

import (
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/zitadel/logging"
	"github.com/zitadel/zitadel/internal/database/dialect"
)

const (
	sslDisabledMode = "disable"
)

type Config struct {
	Host            string
	Port            int32
	Database        string
	MaxOpenConns    uint32
	MaxIdleConns    uint32
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
	User            User
	Admin           User

	//Additional options to be appended as options=<Options>
	//The value will be taken as is. Multiple options are space separated.
	Options string
}

func (c *Config) MatchName(name string) bool {
	for _, key := range []string{"pg", "postgres"} {
		if strings.TrimSpace(strings.ToLower(name)) == key {
			return true
		}
	}
	return false
}

func (c *Config) Decode(configs []interface{}) (dialect.Connector, error) {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook:       mapstructure.StringToTimeDurationHookFunc(),
		WeaklyTypedInput: true,
		Result:           c,
	})
	if err != nil {
		return nil, err
	}

	for _, config := range configs {
		if err = decoder.Decode(config); err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (c *Config) Connect(useAdmin bool) (*sql.DB, error) {
	logging.Warn("postgres is currently in beta")
	db, err := sql.Open("pgx", c.String(useAdmin))
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(int(c.MaxOpenConns))
	db.SetMaxIdleConns(int(c.MaxIdleConns))
	db.SetConnMaxLifetime(c.MaxConnLifetime)
	db.SetConnMaxIdleTime(c.MaxConnIdleTime)

	return db, nil
}

func (c *Config) DatabaseName() string {
	return c.Database
}

func (c *Config) Username() string {
	return c.User.Username
}

func (c *Config) Password() string {
	return c.User.Password
}

func (c *Config) Type() string {
	return "postgres"
}

func (c *Config) Timetravel(time.Duration) string {
	return ""
}

type User struct {
	Username string
	Password string
	SSL      SSL
}

type SSL struct {
	// type of connection security
	Mode string
	// RootCert Path to the CA certificate
	RootCert string
	// Cert Path to the client certificate
	Cert string
	// Key Path to the client private key
	Key string
}

func (s *Config) checkSSL(user User) {
	if user.SSL.Mode == sslDisabledMode || user.SSL.Mode == "" {
		user.SSL = SSL{Mode: sslDisabledMode}
	}
}

func (c Config) String(useAdmin bool) string {
	user := c.User
	if useAdmin {
		user = c.Admin
	}
	c.checkSSL(user)
	fields := []string{
		"host=" + c.Host,
		"port=" + strconv.Itoa(int(c.Port)),
		"user=" + user.Username,
		"application_name=zitadel",
		"sslmode=" + user.SSL.Mode,
	}
	if c.Options != "" {
		fields = append(fields, "options="+c.Options)
	}
	if user.Password != "" {
		fields = append(fields, "password="+user.Password)
	}
	if !useAdmin {
		fields = append(fields, "dbname="+c.Database)
	} else {
		fields = append(fields, "dbname=postgres")
	}
	if user.SSL.Mode != sslDisabledMode {
		if user.SSL.RootCert != "" {
			fields = append(fields, "sslrootcert="+user.SSL.RootCert)
		}
		if user.SSL.Cert != "" {
			fields = append(fields, "sslcert="+user.SSL.Cert)
		}
		if user.SSL.Key != "" {
			fields = append(fields, "sslkey="+user.SSL.Key)
		}
	}

	return strings.Join(fields, " ")
}
