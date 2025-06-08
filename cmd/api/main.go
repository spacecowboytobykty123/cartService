package main

import (
	"cartService/internal/app/grpcapp"
	crtgrpc "cartService/internal/clients/subscriptions/grpc"
	"cartService/internal/clients/toys/grpc"
	"cartService/internal/jsonlog"
	"cartService/internal/services/cart"
	"cartService/storage/postgres"
	"context"
	"flag"
	"fmt"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	_ "github.com/lib/pq"
	cart_v1_crt "github.com/spacecowboytobykty123/protoCart/proto/gen/go/cart"
	_ "google.golang.org/grpc"
	grpc2 "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	_ "google.golang.org/grpc/credentials/insecure"
	"net/http"
	_ "net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

const version = "1.0.0"

type StorageDetails struct {
	DSN          string
	MaxOpenConns int
	MaxIdleConns int
	MaxIdleTime  string
}

type Client struct {
	Address      int           `yaml:"address"`
	Timeout      time.Duration `yaml:"timeout"`
	RetriesCount int           `yaml:"retries_count"`
	insecure     bool          `yaml:"insecure"`
}

type ClientsConfig struct {
	Subs Client `yaml:"subs"`
	Toys Client `yaml:"toys"`
}

type GRPCConfig struct {
	Port    int
	Timeout time.Duration
}

type Config struct {
	env       string
	DB        StorageDetails
	GRPC      GRPCConfig
	TokenTTL  time.Duration
	Clients   ClientsConfig
	AppSecret string
}

type Application struct {
	GRPCSrv *grpcapp.App
}

func main() {
	var cfg Config

	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	pass := os.Getenv("DB_PASSWORD")
	name := os.Getenv("DB_NAME")

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&client_encoding=UTF8", user, pass, host, port, name)

	flag.StringVar(&cfg.DB.DSN, "db-dsn", dsn, "PostgresSQL DSN")
	flag.IntVar(&cfg.DB.MaxOpenConns, "db-max-open-conns", 25, "PostgresSQL max open connections")
	flag.IntVar(&cfg.DB.MaxIdleConns, "db-max-Idle-conns", 25, "PostgresSQL max Idle connections")
	flag.StringVar(&cfg.DB.MaxIdleTime, "db-max-Idle-time", "15m", "PostgresSQl max Idle time")

	flag.IntVar(&cfg.GRPC.Port, "grpc-port", 5000, "grpc-port")
	flag.DurationVar(&cfg.TokenTTL, "token-ttl", time.Hour, "GRPC's work duration")
	flag.IntVar(&cfg.Clients.Subs.Address, "sub-client-addr", 3000, "sub-port")
	flag.IntVar(&cfg.Clients.Toys.Address, "toys-client-addr", 9000, "toy-port")
	logger := jsonlog.New(os.Stdout, jsonlog.LevelInfo)
	subsClient, err := crtgrpc.New(context.Background(), logger, cfg.Clients.Subs.Address, cfg.Clients.Subs.Timeout, cfg.Clients.Subs.RetriesCount)
	toyClient, err := grpc.New(context.Background(), logger, cfg.Clients.Subs.Timeout, cfg.Clients.Toys.Address)

	if err != nil {
		logger.PrintError(err, map[string]string{
			"message": "failed ot init subs client",
		})
		os.Exit(1)
	}

	flag.Parse()

	app := New(logger, cfg.GRPC.Port, cfg, cfg.TokenTTL, subsClient, toyClient)

	logger.PrintInfo("connection pool established", map[string]string{
		"port": strconv.Itoa(cfg.GRPC.Port),
	})
	go app.GRPCSrv.MustRun()
	go runHttp(cfg.GRPC.Port, logger)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	sign := <-stop
	logger.PrintInfo("stopping application", map[string]string{
		"signal": sign.String(),
	})

	app.GRPCSrv.Stop()

}

func New(log *jsonlog.Logger, grpcPort int, cfg Config, tokenTTL time.Duration, subsClient *crtgrpc.Client, toyClient *grpc.ToyClient) *Application {
	dbCfg := postgres.StorageDetails(cfg.DB)
	db, err := postgres.OpenDB(dbCfg)
	if err != nil {
		log.PrintFatal(err, nil)
	}

	//defer db.Close()

	orderService := cart.New(log, db, tokenTTL, subsClient, toyClient)
	grpcApp := grpcapp.New(log, grpcPort, orderService)

	return &Application{GRPCSrv: grpcApp}
}

func runHttp(grpcPort int, logger *jsonlog.Logger) {
	ctx := context.Background()
	mux := runtime.NewServeMux()
	opts := []grpc2.DialOption{
		grpc2.WithTransportCredentials(insecure.NewCredentials()),
	}

	endpoint := "localhost:" + strconv.Itoa(grpcPort)
	if err := cart_v1_crt.RegisterCartHandlerFromEndpoint(ctx, mux, endpoint, opts); err != nil {
		logger.PrintFatal(err, map[string]string{
			"message": "failed to start HTTP gateway",
			"method":  "main.runHTTP",
		})
	}
	fs := http.FileServer(http.Dir("C:\\Users\\Еркебулан\\GolandProjects\\protoCart\\proto\\gen\\swagger"))
	http.Handle("/swagger/", http.StripPrefix("/swagger/", fs))
	http.Handle("/", mux)

	logger.PrintInfo("HTTP REST gateway and Swagger docs started", map[string]string{
		"port": "8080",
	})

	if err := http.ListenAndServe(":8080", mux); err != nil {
		logger.PrintFatal(err, map[string]string{
			"message": "HTTP gateway crashed",
		})
	}
}
