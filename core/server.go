package core

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"ctaf-fz/config"
	"ctaf-fz/global"
	"ctaf-fz/initialize"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Run 启动 HTTP 服务、后台任务，并在收到退出信号时优雅关闭。
func Run(engine *gin.Engine, cfg *config.Config, app *initialize.App) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	jobCtx, cancelJobs := context.WithCancel(context.Background())
	defer cancelJobs()
	if app.CleanLogJob != nil {
		app.CleanLogJob.Start(jobCtx)
	}
	if app.RuleDownloadJob != nil {
		app.RuleDownloadJob.Start(jobCtx)
	}
	if app.StatusReportJob != nil {
		app.StatusReportJob.Start(jobCtx)
	}

	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      engine,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		global.Logger.Info("ctaf-fz listening", zap.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		global.Logger.Info("shutdown signal received")
		cancelJobs()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
		defer cancel()
		err := server.Shutdown(shutdownCtx)
		closeErr := app.Store.Close()
		if err != nil {
			global.Logger.Error("server shutdown failed", zap.Error(err))
			return err
		}
		if closeErr != nil {
			global.Logger.Error("store close failed", zap.Error(closeErr))
			return closeErr
		}
		global.Logger.Info("server shutdown complete")
		return nil
	case err := <-errCh:
		cancelJobs()
		closeErr := app.Store.Close()
		if err != nil {
			global.Logger.Error("server stopped", zap.Error(err))
			return err
		}
		if closeErr != nil {
			global.Logger.Error("store close failed", zap.Error(closeErr))
			return closeErr
		}
		return nil
	}
}
