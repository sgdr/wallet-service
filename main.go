package main

import (
	"context"
	"flag"
	"github.com/sgdr/wallet-service/internal/account"
	"github.com/sgdr/wallet-service/internal/datasource"
	"github.com/sgdr/wallet-service/internal/db"
	"github.com/sgdr/wallet-service/internal/payment"
	"html/template"
	"net/http"
	"os"
	"os/signal"

	logKit "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/sgdr/wallet-service/internal/config"
	"github.com/sgdr/wallet-service/internal/logger"
)

func main() {
	ctx := context.Background()
	log := logger.Init()
	level.Info(log).Log("msg", "wallet's service is starting...")

	var configPath string
	flag.StringVar(&configPath, "config-path", "./config/config.yml", "A path to config file")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		level.Error(log).Log("msg", "loading of configuration fails "+err.Error())
		return
	}

	dataSource, err := datasource.New(ctx, cfg.Db)
	if err != nil {
		level.Error(log).Log("msg", "creation of data source fails "+err.Error())
		return
	}
	if err := db.CreateDbStructure(ctx, dataSource, "./internal/db/update_db_structure.sql"); err != nil {
		level.Error(log).Log("msg", "db structure's creation fails "+err.Error())
		return
	}
	accountRep := account.NewRepository(dataSource)
	paymentRep := payment.NewRepository(dataSource)
	accountService := account.NewService(accountRep)
	paymentService := payment.NewService(paymentRep, accountRep)
	router := mux.NewRouter()
	router.HandleFunc("/doc", getDoc).Methods("GET")
	router.HandleFunc("/swagger.yml", swagger).Methods("GET")
	apiSubRouter := router.PathPrefix("/api/v1/").Subrouter()
	apiSubRouter.Use(addRequestIdToLogMiddleware)
	apiSubRouter.HandleFunc("/accounts/all", account.AllAccountsHandler(accountService)).Methods("GET")
	apiSubRouter.HandleFunc("/payments/all", payment.AllPaymentsForClient(paymentService)).Methods("GET")
	apiSubRouter.HandleFunc("/payment", payment.CreatePayment(paymentService)).Methods("POST")
	httpServerExternal := http.Server{Addr: ":" + cfg.HttpPort, Handler: router}
	go func() {
		if err := httpServerExternal.ListenAndServe(); err != nil {
		}
	}()

	stop := make(chan os.Signal)
	signal.Notify(stop, os.Interrupt)
	<-stop

	if err := httpServerExternal.Shutdown(ctx); err != nil {
	}
}

func addRequestIdToLogMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		l := logger.FromContext(ctx)
		l = logKit.With(l, "request_id", uuid.New().String())
		ctx = logger.ToContext(ctx, l)
		r = r.WithContext(ctx)
		// Call the next handler
		handler.ServeHTTP(w, r)
	})

}

func getDoc(w http.ResponseWriter, r *http.Request) {
	t, _ := template.ParseFiles("./internal/api/index.html")
	t.Execute(w, nil)
}

func swagger(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	http.ServeFile(w, r, "./internal/api/swagger.yml")
}
