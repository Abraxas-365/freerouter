package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Abraxas-365/freerouter/internal/config"
	"github.com/Abraxas-365/freerouter/internal/logx"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
		"github.com/Abraxas-365/freerouter/internal/fsx"
		"github.com/Abraxas-365/freerouter/internal/fsx/fsxlocal"
		"github.com/Abraxas-365/freerouter/internal/fsx/fsxs3"
		awsConfig "github.com/aws/aws-sdk-go-v2/config"
		"github.com/aws/aws-sdk-go-v2/service/s3"
		"github.com/Abraxas-365/freerouter/internal/iam/iamcontainer"
		"github.com/Abraxas-365/freerouter/internal/ai/aicontainer"
		"github.com/Abraxas-365/freerouter/internal/kernel"
		"github.com/Abraxas-365/freerouter/internal/jobx"
		"github.com/Abraxas-365/freerouter/internal/jobx/jobxredis"
		"github.com/Abraxas-365/freerouter/internal/notifx"
		"github.com/Abraxas-365/freerouter/internal/notifx/notifxses"
		"github.com/Abraxas-365/freerouter/internal/notifx/notifxconsole"
		"github.com/aws/aws-sdk-go-v2/service/ses"
	// manifesto:container-imports
)

// Container holds shared infrastructure and composed module containers.
type Container struct {
	Config *config.Config

	// Infrastructure
	DB    *sqlx.DB
	Redis *redis.Client

	// Bounded-context containers
		FileSystem fsx.FileSystem
	S3Client   *s3.Client
		IAM *iamcontainer.Container
		AI  *aicontainer.Container
		JobClient *jobx.Client
		NotifxClient *notifx.Client
	// manifesto:container-fields
}

func NewContainer(cfg *config.Config) *Container {
	logx.Info("Initializing application container...")

	c := &Container{Config: cfg}

	c.initInfrastructure()
	c.initModules()

	logx.Info("Application container initialized")
	return c
}

// ---------------------------------------------------------------------------
// Infrastructure
// ---------------------------------------------------------------------------

func (c *Container) initInfrastructure() {
	logx.Info("Initializing infrastructure...")

	// Database
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Config.Database.Host,
		c.Config.Database.Port,
		c.Config.Database.User,
		c.Config.Database.Password,
		c.Config.Database.Name,
		c.Config.Database.SSLMode,
	)

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		logx.Fatalf("Failed to connect to database: %v", err)
	}
	db.SetMaxOpenConns(c.Config.Database.MaxOpenConns)
	db.SetMaxIdleConns(c.Config.Database.MaxIdleConns)
	db.SetConnMaxLifetime(c.Config.Database.ConnMaxLifetime)
	c.DB = db
	logx.Info("  Database connected")

	// Redis
	c.Redis = redis.NewClient(&redis.Options{
		Addr:     c.Config.Redis.Address(),
		Password: c.Config.Redis.Password,
		DB:       c.Config.Redis.DB,
	})
	if _, err := c.Redis.Ping(context.Background()).Result(); err != nil {
		logx.Fatalf("Failed to connect to Redis: %v (Redis is required)", err)
	}
	logx.Info("  Redis connected")

	logx.Info("Infrastructure initialized")
}

// ---------------------------------------------------------------------------
// Module composition
// ---------------------------------------------------------------------------

func (c *Container) initModules() {
	logx.Info("Initializing modules...")

		c.initFileStorage()

		c.IAM = iamcontainer.New(iamcontainer.Deps{
		DB:                 c.DB,
		Redis:              c.Redis,
		Cfg:                c.Config,
		OTPNotifier:        NewConsoleNotifier(),
		InvitationNotifier: NewConsoleInvitationNotifier(),
	})

	c.AI = aicontainer.New(aicontainer.Deps{
		DB:  c.DB,
		Cfg: c.Config,
	})

		c.initJobx()

		c.initNotifx()

	// manifesto:module-init
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

func (c *Container) StartBackgroundServices(ctx context.Context) {
	logx.Info("Starting background services...")
		c.IAM.StartBackgroundServices(ctx)
		go c.JobClient.Start(ctx)
	// manifesto:background-start
}

func (c *Container) Cleanup() {
	logx.Info("Cleaning up resources...")

	if c.AI != nil {
		c.AI.Close()
	}

	if c.DB != nil {
		if err := c.DB.Close(); err != nil {
			logx.Errorf("Error closing database: %v", err)
		} else {
			logx.Info("  Database connection closed")
		}
	}

	if c.Redis != nil {
		if err := c.Redis.Close(); err != nil {
			logx.Errorf("Error closing Redis: %v", err)
		} else {
			logx.Info("  Redis connection closed")
		}
	}

	logx.Info("Cleanup complete")
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func (c *Container) initFileStorage() {
	storageMode := getEnv("STORAGE_MODE", "local")

	switch storageMode {
	case "s3":
		awsRegion := getEnv("AWS_REGION", "us-east-1")
		awsBucket := getEnv("AWS_BUCKET", "freerouter-uploads")

		cfg, err := awsConfig.LoadDefaultConfig(context.TODO(), awsConfig.WithRegion(awsRegion))
		if err != nil {
			logx.Fatalf("Unable to load AWS SDK config: %v", err)
		}
		c.S3Client = s3.NewFromConfig(cfg)
		c.FileSystem = fsxs3.NewS3FileSystem(c.S3Client, awsBucket, "")
		logx.Infof("  S3 file system configured (bucket: %s, region: %s)", awsBucket, awsRegion)

	case "local":
		uploadDir := getEnv("UPLOAD_DIR", "./uploads")
		localFS, err := fsxlocal.NewLocalFileSystem(uploadDir)
		if err != nil {
			logx.Fatalf("Failed to initialize local file system: %v", err)
		}
		c.FileSystem = localFS
		logx.Infof("  Local file system configured (path: %s)", localFS.GetBasePath())

	default:
		logx.Fatalf("Unknown STORAGE_MODE: %s (use 'local' or 's3')", storageMode)
	}
}

// ConsoleNotifier implements the NotificationService interface
// by printing OTP codes to the terminal/console
type ConsoleNotifier struct{}

// NewConsoleNotifier creates a new console-based OTP notifier
func NewConsoleNotifier() *ConsoleNotifier {
	return &ConsoleNotifier{}
}

// SendOTP prints the OTP code to the terminal
func (n *ConsoleNotifier) SendOTP(ctx context.Context, contact string, code string) error {
	fmt.Println("\n" + repeatString("=", 60))
	fmt.Println("📧 OTP NOTIFICATION (Console Output)")
	fmt.Println(repeatString("=", 60))
	fmt.Printf("📨 To: %s\n", contact)
	fmt.Printf("🔐 Code: %s\n", code)
	fmt.Println(repeatString("=", 60))
	fmt.Println("⚠️  This is console output for development only")
	fmt.Println("⚠️  In production, configure email service in config")
	fmt.Println(repeatString("=", 60) + "\n")

	logx.Infof("📧 OTP sent to %s: %s", contact, code)
	return nil
}

// ConsoleInvitationNotifier implements invitation.NotificationService
// by printing invitation details to the terminal/console
type ConsoleInvitationNotifier struct{}

func NewConsoleInvitationNotifier() *ConsoleInvitationNotifier {
	return &ConsoleInvitationNotifier{}
}

func (n *ConsoleInvitationNotifier) SendInvitation(ctx context.Context, email string, token string, tenantID kernel.TenantID, invitedBy kernel.UserID) error {
	fmt.Println("\n" + repeatString("=", 60))
	fmt.Println("📧 INVITATION NOTIFICATION (Console Output)")
	fmt.Println(repeatString("=", 60))
	fmt.Printf("📨 To: %s\n", email)
	fmt.Printf("🔗 Token: %s\n", token)
	fmt.Printf("🏢 Tenant: %s\n", tenantID)
	fmt.Printf("👤 Invited by: %s\n", invitedBy)
	fmt.Println(repeatString("=", 60))
	fmt.Println("⚠️  This is console output for development only")
	fmt.Println("⚠️  In production, configure notifx for email delivery")
	fmt.Println(repeatString("=", 60) + "\n")

	logx.Infof("📧 Invitation sent to %s (token: %s...)", email, token[:8])
	return nil
}

func (c *Container) initJobx() {
	queue := jobxredis.NewRedisQueue(c.Redis)
	c.JobClient = jobx.NewClient(queue,
		jobx.WithConcurrency(c.Config.Jobx.Concurrency),
		jobx.WithQueues(c.Config.Jobx.Queues...),
		jobx.WithPollInterval(c.Config.Jobx.PollInterval),
		jobx.WithShutdownTimeout(c.Config.Jobx.ShutdownTimeout),
		jobx.WithDequeueTimeout(c.Config.Jobx.DequeueTimeout),
		jobx.WithDefaultRetryDelay(c.Config.Jobx.DefaultRetryDelay),
	)
	logx.Info("  Job queue configured")
}

func (c *Container) initNotifx() {
	var provider notifx.EmailSender

	switch c.Config.Notifx.Provider {
	case "ses":
		awsCfg, err := awsConfig.LoadDefaultConfig(context.TODO(),
			awsConfig.WithRegion(c.Config.Notifx.AWSRegion))
		if err != nil {
			logx.Fatalf("Unable to load AWS config for notifx: %v", err)
		}
		sesClient := ses.NewFromConfig(awsCfg)
		provider = notifxses.NewSESProvider(sesClient, c.Config.Notifx.FromAddress)
		logx.Infof("  Notifx: SES provider (region: %s)", c.Config.Notifx.AWSRegion)

	default:
		provider = notifxconsole.NewConsoleProvider()
		logx.Info("  Notifx: console provider (dev mode)")
	}

	c.NotifxClient = notifx.NewClient(provider)
}

// manifesto:container-helpers
