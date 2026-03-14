package bootstrap

import (
	infragin "github.com/jonesrussell/north-cloud/infrastructure/gin"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/api"
	"github.com/jonesrussell/north-cloud/source-manager/internal/config"
	"github.com/jonesrussell/north-cloud/source-manager/internal/database"
	"github.com/jonesrussell/north-cloud/source-manager/internal/events"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
	"github.com/jonesrussell/north-cloud/source-manager/internal/services"
	"github.com/jonesrussell/north-cloud/source-manager/internal/services/osrm"
)

// SetupHTTPServer creates and configures the HTTP server.
func SetupHTTPServer(
	cfg *config.Config,
	db *database.DB,
	publisher *events.Publisher,
	log infralogger.Logger,
) *infragin.Server {
	sourceRepo := repository.NewSourceRepository(db.DB(), log)
	communityRepo := repository.NewCommunityRepository(db.DB(), log)
	personRepo := repository.NewPersonRepository(db.DB(), log)
	bandOfficeRepo := repository.NewBandOfficeRepository(db.DB(), log)
	verificationRepo := repository.NewVerificationRepository(db.DB(), log)
	dictionaryRepo := repository.NewDictionaryRepository(db.DB(), log)
	travelTimeRepo := repository.NewTravelTimeRepository(db.DB(), log)

	osrmClient := osrm.NewClient(cfg.OSRM.BaseURL, log)
	travelTimeSvc := services.NewTravelTimeService(osrmClient, travelTimeRepo, communityRepo, log)

	return api.NewServer(
		sourceRepo, communityRepo, personRepo, bandOfficeRepo,
		verificationRepo, dictionaryRepo, travelTimeSvc, cfg, log, publisher,
	)
}
