package acceptance

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cucumber/godog"
	"github.com/gkampitakis/go-snaps/snaps"

	"github.com/enterprise-contract/ec-cli/acceptance/cli"
	"github.com/enterprise-contract/ec-cli/acceptance/conftest"
	"github.com/enterprise-contract/ec-cli/acceptance/crypto"
	"github.com/enterprise-contract/ec-cli/acceptance/git"
	"github.com/enterprise-contract/ec-cli/acceptance/image"
	"github.com/enterprise-contract/ec-cli/acceptance/kubernetes"
	"github.com/enterprise-contract/ec-cli/acceptance/log"
	"github.com/enterprise-contract/ec-cli/acceptance/pipeline"
	"github.com/enterprise-contract/ec-cli/acceptance/registry"
	"github.com/enterprise-contract/ec-cli/acceptance/rekor"
	"github.com/enterprise-contract/ec-cli/acceptance/tekton"
	"github.com/enterprise-contract/ec-cli/acceptance/testenv"
	"github.com/enterprise-contract/ec-cli/acceptance/tuf"
	"github.com/enterprise-contract/ec-cli/acceptance/wiremock"
)

var persist = flag.Bool("persist", false, "persist the stubbed environment to facilitate debugging")
var restore = flag.Bool("restore", false, "restore last persisted environment")
var noColors = flag.Bool("no-colors", false, "disable colored output")
var tags = flag.String("tags", "", "select scenarios to run based on tags")
var seed = flag.Int64("seed", -1, "random seed to use for the tests")

var junitReportPath = os.Getenv("JUNIT_REPORT")

func InitializeScenario(ctx *godog.ScenarioContext) {
	cli.AddStepsTo(ctx)
	crypto.AddStepsTo(ctx)
	git.AddStepsTo(ctx)
	image.AddStepsTo(ctx)
	kubernetes.AddStepsTo(ctx)
	registry.AddStepsTo(ctx)
	rekor.AddStepsTo(ctx)
	tekton.AddStepsTo(ctx)
	wiremock.AddStepsTo(ctx)
	pipeline.AddStepsTo(ctx)
	conftest.AddStepsTo(ctx)
	tuf.AddStepsTo(ctx)

	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		logger, ctx := log.LoggerFor(ctx)
		logger.Name(sc.Name)
		return context.WithValue(ctx, testenv.Scenario, sc), nil
	})

	ctx.After(func(ctx context.Context, scenario *godog.Scenario, scenarioErr error) (context.Context, error) {
		_, err := testenv.Persist(ctx)
		return ctx, err
	})
}

func initializeSuite(ctx context.Context) func(*godog.TestSuiteContext) {
	return func(tsc *godog.TestSuiteContext) {
		kubernetes.InitializeSuite(ctx, tsc)
	}
}

func setupContext(t *testing.T) context.Context {
	ctx := context.WithValue(context.Background(), testenv.TestingT, t)
	ctx = context.WithValue(ctx, testenv.PersistStubEnvironment, *persist)
	ctx = context.WithValue(ctx, testenv.RestoreStubEnvironment, *restore)
	ctx = context.WithValue(ctx, testenv.NoColors, *noColors)
	return ctx
}

func TestFeatures(t *testing.T) {
	flag.Parse()

	if junitReportPath != "" {
		fmt.Println("📄 Will write JUnit report to:", junitReportPath)
	}

	featuresDir, err := filepath.Abs("../features")
	if err != nil {
		t.Fatal(err)
	}

	files, err := filepath.Glob(filepath.Join(featuresDir, "*.feature"))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("🔍 Found %d feature files in %s\n", len(files), featuresDir)
	if len(files) == 0 {
		t.Fatal("❌ No feature files found")
	}

	ctx := setupContext(t)

	opts := godog.Options{
		Format:         "pretty",
		Paths:          []string{featuresDir},
		Randomize:      *seed,
		Concurrency:    runtime.NumCPU(),
		TestingT:       t,
		DefaultContext: ctx,
		Tags:           *tags,
		NoColors:       *noColors,
		Output:         os.Stdout,
	}

	if junitReportPath != "" {
		f, err := os.Create(junitReportPath)
		if err != nil {
			t.Fatalf("failed to create JUnit report: %v", err)
		}
		defer f.Close()
		opts.Format = "junit"
		opts.Output = f
	}

	suite := godog.TestSuite{
		Name:                "ec-cli",
		ScenarioInitializer: InitializeScenario,
		Options:             &opts,
	}

	if suite.Run() != 0 {
		t.Fail()
	}

	if junitReportPath != "" {
		if _, err := os.Stat(junitReportPath); os.IsNotExist(err) {
			t.Logf("⚠️ JUnit report NOT created at: %s", junitReportPath)
		} else {
			t.Logf("✅ JUnit report created at: %s", junitReportPath)
		}
	}
}

func TestMain(t *testing.M) {
	v := t.Run()

	// After all tests have run `go-snaps` can check for not used snapshots
	snaps.Clean(t)

	os.Exit(v)
}
