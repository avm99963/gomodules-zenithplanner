package scheduler

import (
	"context"
	"log"
	"gomodules.avm99963.com/zenithplanner/internal/config"
	"gomodules.avm99963.com/zenithplanner/internal/sync"

	"github.com/robfig/cron/v3"
)

// Scheduler manages background tasks.
type Scheduler struct {
	cron   *cron.Cron
	syncer *sync.Syncer
	cfg    *config.AppConfig
}

// NewScheduler creates and configures a new task scheduler.
func NewScheduler(syncer *sync.Syncer, appCfg *config.AppConfig) *Scheduler {
	cronLogger := cron.PrintfLogger(log.New(log.Writer(), "CRON: ", log.LstdFlags))
	c := cron.New(cron.WithLogger(cronLogger))

	s := &Scheduler{
		cron:   c,
		syncer: syncer,
		cfg:    appCfg,
	}

	s.registerTasks()

	return s
}

// registerTasks adds the scheduled jobs.
func (s *Scheduler) registerTasks() {
	if s.cfg.Scheduler.EnableHorizonMaintenance {
		_, err := s.cron.AddFunc(s.cfg.Scheduler.HorizonMaintenanceCron, s.runHorizonMaintenance)
		if err != nil {
			log.Printf("Error scheduling horizon maintenance: %v", err)
		} else {
			log.Println("Scheduled horizon maintenance task (daily at 2 AM).")
		}
	}

	if s.cfg.Scheduler.EnablePeriodicFullSync {
		_, err := s.cron.AddFunc(s.cfg.Scheduler.PeriodicFullSyncCron, s.runWeeklyFullSync)
		if err != nil {
			log.Printf("Error scheduling weekly full sync: %v", err)
		} else {
			log.Println("Scheduled weekly full sync task (Sunday at 3 AM).")
		}
	}

	if s.cfg.EnableCalendarSubscription {
		_, err := s.cron.AddFunc(s.cfg.Scheduler.CalendarSubscriptionMaintenanceCron, s.runChannelRenewal)
		if err != nil {
			log.Printf("Error scheduling channel renewal: %v", err)
		} else {
			log.Println("Scheduled channel renewal task (daily at 1 AM).")
		}
	}
}

// runHorizonMaintenance is a wrapper function called by the cron scheduler.
func (s *Scheduler) runHorizonMaintenance() {
	log.Println("Scheduler: Running daily horizon maintenance task...")
	ctx := context.Background()
	err := s.syncer.RunHorizonMaintenanceTask(ctx)
	if err != nil {
		log.Printf("Error during scheduled horizon maintenance: %v", err)
	} else {
		log.Println("Scheduler: Horizon maintenance task finished.")
	}
}

// runWeeklyFullSync is a wrapper function called by the cron scheduler.
func (s *Scheduler) runWeeklyFullSync() {
	log.Println("Scheduler: Running weekly full sync task...")
	ctx := context.Background()
	err := s.syncer.RunFullSync(ctx)
	if err != nil {
		log.Printf("Error during scheduled weekly full sync: %v", err)
	} else {
		log.Println("Scheduler: Weekly full sync task finished.")
	}
}

// runChannelRenewal is a wrapper function called by the cron scheduler.
func (s *Scheduler) runChannelRenewal() {
	log.Println("Scheduler: Running daily channel renewal task...")
	ctx := context.Background()
	err := s.syncer.RunChannelRenewalTask(ctx)
	if err != nil {
		log.Printf("Error during scheduled channel renewal: %v", err)
	} else {
		log.Println("Scheduler: Channel renewal task finished.")
	}
}

// Start begins the cron scheduler in a non-blocking way.
func (s *Scheduler) Start() {
	log.Println("Starting background task scheduler...")
	s.cron.Start()
}

// Stop gracefully stops the cron scheduler, waiting for running jobs to finish.
func (s *Scheduler) Stop() context.Context {
	log.Println("Stopping background task scheduler (waiting for jobs to complete)...")
	return s.cron.Stop()
}
