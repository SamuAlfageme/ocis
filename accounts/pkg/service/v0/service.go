package service

import (
	"context"
	"errors"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/owncloud/ocis/accounts/pkg/indexer"
	"github.com/owncloud/ocis/accounts/pkg/storage"

	mclient "github.com/micro/go-micro/v2/client"
	"github.com/owncloud/ocis/accounts/pkg/config"
	idxerrs "github.com/owncloud/ocis/accounts/pkg/indexer/errors"
	"github.com/owncloud/ocis/accounts/pkg/proto/v0"
	"github.com/owncloud/ocis/ocis-pkg/log"
	"github.com/owncloud/ocis/ocis-pkg/roles"
	settings "github.com/owncloud/ocis/settings/pkg/proto/v0"
	settings_svc "github.com/owncloud/ocis/settings/pkg/service/v0"
)

// userDefaultGID is the default integer representing the "users" group.
const userDefaultGID = 30000

// New returns a new instance of Service
func New(opts ...Option) (s *Service, err error) {
	options := newOptions(opts...)
	logger := options.Logger
	cfg := options.Config

	roleService := options.RoleService
	if roleService == nil {
		// https://github.com/owncloud/ocis-proxy/issues/38
		// TODO this won't work with a registry other than mdns. Look into Micro's client initialization.
		roleService = settings.NewRoleService("com.owncloud.api.settings", mclient.DefaultClient)
	}
	roleManager := options.RoleManager
	if roleManager == nil {
		m := roles.NewManager(
			roles.CacheSize(1024),
			roles.CacheTTL(time.Hour*24*7),
			roles.Logger(options.Logger),
			roles.RoleService(roleService),
		)
		roleManager = &m
	}

	s = &Service{
		id:          cfg.GRPC.Namespace + "." + cfg.Server.Name,
		log:         logger,
		Config:      cfg,
		RoleService: roleService,
		RoleManager: roleManager,
		repo:        createMetadataStorage(cfg, logger),
	}

	if s.index, err = s.buildIndex(); err != nil {
		return nil, err
	}

	if err = s.createDefaultAccounts(); err != nil {
		return nil, err
	}

	if err = s.createDefaultGroups(); err != nil {
		return nil, err
	}
	// TODO watch folders for new records
	return
}

func (s Service) buildIndex() (*indexer.Indexer, error) {
	idx := indexer.CreateIndexer(s.Config)

	if err := recreateContainers(idx, s.Config); err != nil {
		return nil, err
	}
	return idx, nil

}

func (s Service) createDefaultAccounts() (err error) {
	accounts := []proto.Account{
		{
			Id:                       "4c510ada-c86b-4815-8820-42cdf82c3d51",
			PreferredName:            "einstein",
			OnPremisesSamAccountName: "einstein",
			Mail:                     "einstein@example.org",
			DisplayName:              "Albert Einstein",
			UidNumber:                20000,
			GidNumber:                30000,
			PasswordProfile: &proto.PasswordProfile{
				Password: "$2a$12$qGho4QmaDn4HifisABQ2ROBc41TUCIBNrqwOg3zPEYednDWkXHOKG",
			},
			AccountEnabled: true,
			MemberOf: []*proto.Group{
				{Id: "509a9dcd-bb37-4f4f-a01a-19dca27d9cfa"}, // users
				{Id: "6040aa17-9c64-4fef-9bd0-77234d71bad0"}, // sailing-lovers
				{Id: "dd58e5ec-842e-498b-8800-61f2ec6f911f"}, // violin-haters
				{Id: "262982c1-2362-4afa-bfdf-8cbfef64a06e"}, // physics-lovers
			},
		},
		{
			Id:                       "f7fbf8c8-139b-4376-b307-cf0a8c2d0d9c",
			PreferredName:            "marie",
			OnPremisesSamAccountName: "marie",
			Mail:                     "marie@example.org",
			DisplayName:              "Marie Curie",
			UidNumber:                20001,
			GidNumber:                30000,
			PasswordProfile: &proto.PasswordProfile{
				Password: "$2a$12$JzfOtyRiGL25w1UpmiPZ1uU7KJURoTMPSRlZQvMB90/1zqvjwEWgO",
			},
			AccountEnabled: true,
			MemberOf: []*proto.Group{
				{Id: "509a9dcd-bb37-4f4f-a01a-19dca27d9cfa"}, // users
				{Id: "7b87fd49-286e-4a5f-bafd-c535d5dd997a"}, // radium-lovers
				{Id: "cedc21aa-4072-4614-8676-fa9165f598ff"}, // polonium-lovers
				{Id: "262982c1-2362-4afa-bfdf-8cbfef64a06e"}, // physics-lovers
			},
		},
		{
			Id:                       "932b4540-8d16-481e-8ef4-588e4b6b151c",
			PreferredName:            "richard",
			OnPremisesSamAccountName: "richard",
			Mail:                     "richard@example.org",
			DisplayName:              "Richard Feynman",
			UidNumber:                20002,
			GidNumber:                30000,
			PasswordProfile: &proto.PasswordProfile{
				Password: "$2a$12$bWMHNfc92rDForMapNle/eJ1fOY0eTeRzpk1EQFUdKGI6UdTktp/a",
			},
			AccountEnabled: true,
			MemberOf: []*proto.Group{
				{Id: "509a9dcd-bb37-4f4f-a01a-19dca27d9cfa"}, // users
				{Id: "a1726108-01f8-4c30-88df-2b1a9d1cba1a"}, // quantum-lovers
				{Id: "167cbee2-0518-455a-bfb2-031fe0621e5d"}, // philosophy-haters
				{Id: "262982c1-2362-4afa-bfdf-8cbfef64a06e"}, // physics-lovers
			},
		},
		// admin user(s)
		{
			Id:                       "058bff95-6708-4fe5-91e4-9ea3d377588b",
			PreferredName:            "moss",
			OnPremisesSamAccountName: "moss",
			Mail:                     "moss@example.org",
			DisplayName:              "Maurice Moss",
			UidNumber:                20003,
			GidNumber:                30000,
			PasswordProfile: &proto.PasswordProfile{
				Password: "$2a$12$TY7jDd1PNbZXzZJMvIFm3eXAL4wjzOl.QXJZ6GKGDpAmUcvHBNc66",
			},
			AccountEnabled: true,
			MemberOf: []*proto.Group{
				{Id: "509a9dcd-bb37-4f4f-a01a-19dca27d9cfa"}, // users
			},
		},
		// technical users for kopano and reva
		{
			Id:                       "820ba2a1-3f54-4538-80a4-2d73007e30bf",
			PreferredName:            "konnectd",
			OnPremisesSamAccountName: "konnectd",
			Mail:                     "idp@example.org",
			DisplayName:              "Kopano Konnectd",
			UidNumber:                10000,
			GidNumber:                15000,
			PasswordProfile: &proto.PasswordProfile{
				Password: "$2a$12$pV8JlGlpM9oo1RaC7kSg/uBTFoTOJ8XfjibFL2U4gOpPKVfbD6pUG",
			},
			AccountEnabled: true,
			MemberOf: []*proto.Group{
				{Id: "34f38767-c937-4eb6-b847-1c175829a2a0"}, // sysusers
			},
		},
		{
			Id:                       "bc596f3c-c955-4328-80a0-60d018b4ad57",
			PreferredName:            "reva",
			OnPremisesSamAccountName: "reva",
			Mail:                     "storage@example.org",
			DisplayName:              "Reva Inter Operability Platform",
			UidNumber:                10001,
			GidNumber:                15000,
			PasswordProfile: &proto.PasswordProfile{
				Password: "$2a$12$CSM.vkX9o7lO/uvid3XieOVtmq5nh91MFZHvHIsfRms3hLzTa2W6.",
			},
			AccountEnabled: true,
			MemberOf: []*proto.Group{
				{Id: "34f38767-c937-4eb6-b847-1c175829a2a0"}, // sysusers
			},
		},
	}
	for i := range accounts {
		if err := s.repo.WriteAccount(context.Background(), &accounts[i]); err != nil {
			return err
		}

		results, err := s.index.Add(&accounts[i])
		if err != nil {
			if idxerrs.IsAlreadyExistsErr(err) {
				continue
			} else {
				return err
			}
		}

		// TODO: can be removed again as soon as we respect the predefined UIDs and GIDs from the account. Then no autoincrement is happening, therefore we don't need to update accounts.
		changed := false
		for _, r := range results {
			if r.Field == "UidNumber" || r.Field == "GidNumber" {
				id, err := strconv.ParseInt(path.Base(r.Value), 10, 0)
				if err != nil {
					return err
				}
				if r.Field == "UidNumber" {
					accounts[i].UidNumber = id
				} else {
					accounts[i].GidNumber = id
				}
				changed = true
			}
		}
		if changed {
			if err := s.repo.WriteAccount(context.Background(), &accounts[i]); err != nil {
				return err
			}
		}
	}

	// set role for admin users and regular users
	assignRoleToUser("058bff95-6708-4fe5-91e4-9ea3d377588b", settings_svc.BundleUUIDRoleAdmin, s.RoleService, s.log)
	for _, accountID := range []string{
		"058bff95-6708-4fe5-91e4-9ea3d377588b", //moss
	} {
		assignRoleToUser(accountID, settings_svc.BundleUUIDRoleAdmin, s.RoleService, s.log)
	}
	for _, accountID := range []string{
		"4c510ada-c86b-4815-8820-42cdf82c3d51", //einstein
		"f7fbf8c8-139b-4376-b307-cf0a8c2d0d9c", //marie
		"932b4540-8d16-481e-8ef4-588e4b6b151c", //richard
	} {
		assignRoleToUser(accountID, settings_svc.BundleUUIDRoleUser, s.RoleService, s.log)
	}
	return nil
}

func (s Service) createDefaultGroups() (err error) {
	groups := []proto.Group{
		{Id: "34f38767-c937-4eb6-b847-1c175829a2a0", GidNumber: 15000, OnPremisesSamAccountName: "sysusers", DisplayName: "Technical users", Description: "A group for technical users. They should not show up in sharing dialogs.", Members: []*proto.Account{
			{Id: "820ba2a1-3f54-4538-80a4-2d73007e30bf"}, // konnectd
			{Id: "bc596f3c-c955-4328-80a0-60d018b4ad57"}, // reva
		}},
		{Id: "509a9dcd-bb37-4f4f-a01a-19dca27d9cfa", GidNumber: 30000, OnPremisesSamAccountName: "users", DisplayName: "Users", Description: "A group every normal user belongs to.", Members: []*proto.Account{
			{Id: "4c510ada-c86b-4815-8820-42cdf82c3d51"}, // einstein
			{Id: "f7fbf8c8-139b-4376-b307-cf0a8c2d0d9c"}, // marie
			{Id: "932b4540-8d16-481e-8ef4-588e4b6b151c"}, // feynman
		}},
		{Id: "6040aa17-9c64-4fef-9bd0-77234d71bad0", GidNumber: 30001, OnPremisesSamAccountName: "sailing-lovers", DisplayName: "Sailing lovers", Members: []*proto.Account{
			{Id: "4c510ada-c86b-4815-8820-42cdf82c3d51"}, // einstein
		}},
		{Id: "dd58e5ec-842e-498b-8800-61f2ec6f911f", GidNumber: 30002, OnPremisesSamAccountName: "violin-haters", DisplayName: "Violin haters", Members: []*proto.Account{
			{Id: "4c510ada-c86b-4815-8820-42cdf82c3d51"}, // einstein
		}},
		{Id: "7b87fd49-286e-4a5f-bafd-c535d5dd997a", GidNumber: 30003, OnPremisesSamAccountName: "radium-lovers", DisplayName: "Radium lovers", Members: []*proto.Account{
			{Id: "f7fbf8c8-139b-4376-b307-cf0a8c2d0d9c"}, // marie
		}},
		{Id: "cedc21aa-4072-4614-8676-fa9165f598ff", GidNumber: 30004, OnPremisesSamAccountName: "polonium-lovers", DisplayName: "Polonium lovers", Members: []*proto.Account{
			{Id: "f7fbf8c8-139b-4376-b307-cf0a8c2d0d9c"}, // marie
		}},
		{Id: "a1726108-01f8-4c30-88df-2b1a9d1cba1a", GidNumber: 30005, OnPremisesSamAccountName: "quantum-lovers", DisplayName: "Quantum lovers", Members: []*proto.Account{
			{Id: "932b4540-8d16-481e-8ef4-588e4b6b151c"}, // feynman
		}},
		{Id: "167cbee2-0518-455a-bfb2-031fe0621e5d", GidNumber: 30006, OnPremisesSamAccountName: "philosophy-haters", DisplayName: "Philosophy haters", Members: []*proto.Account{
			{Id: "932b4540-8d16-481e-8ef4-588e4b6b151c"}, // feynman
		}},
		{Id: "262982c1-2362-4afa-bfdf-8cbfef64a06e", GidNumber: 30007, OnPremisesSamAccountName: "physics-lovers", DisplayName: "Physics lovers", Members: []*proto.Account{
			{Id: "4c510ada-c86b-4815-8820-42cdf82c3d51"}, // einstein
			{Id: "f7fbf8c8-139b-4376-b307-cf0a8c2d0d9c"}, // marie
			{Id: "932b4540-8d16-481e-8ef4-588e4b6b151c"}, // feynman
		}},
	}
	for i := range groups {
		if err := s.repo.WriteGroup(context.Background(), &groups[i]); err != nil {
			return err
		}

		results, err := s.index.Add(&groups[i])
		if err != nil {
			if idxerrs.IsAlreadyExistsErr(err) {
				continue
			} else {
				return err
			}
		}

		// TODO: can be removed again as soon as we respect the predefined GIDs from the group. Then no autoincrement is happening, therefore we don't need to update groups.
		for _, r := range results {
			if r.Field == "GidNumber" {
				gid, err := strconv.ParseInt(path.Base(r.Value), 10, 0)
				if err != nil {
					return err
				}
				groups[i].GidNumber = gid
				if err := s.repo.WriteGroup(context.Background(), &groups[i]); err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}

func assignRoleToUser(accountID, roleID string, rs settings.RoleService, logger log.Logger) (ok bool) {
	_, err := rs.AssignRoleToUser(context.Background(), &settings.AssignRoleToUserRequest{
		AccountUuid: accountID,
		RoleId:      roleID,
	})
	if err != nil {
		logger.Error().Err(err).Str("accountID", accountID).Str("roleID", roleID).Msg("could not set role for account")
		return false
	}
	return true
}

func createMetadataStorage(cfg *config.Config, logger log.Logger) storage.Repo {
	// for now we detect the used storage implementation based on which storage is configured
	// the config with defaults needs to be checked last
	if cfg.Repo.Disk.Path != "" {
		return storage.NewDiskRepo(cfg, logger)
	}
	repo, err := storage.NewCS3Repo(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("cs3 storage was configured but failed to start")
	}
	return repo
}

// Service implements the AccountsServiceHandler interface
type Service struct {
	id          string
	log         log.Logger
	Config      *config.Config
	index       *indexer.Indexer
	RoleService settings.RoleService
	RoleManager *roles.Manager
	repo        storage.Repo
}

func cleanupID(id string) (string, error) {
	id = filepath.Clean(id)
	if id == "." || strings.Contains(id, "/") {
		return "", errors.New("invalid id " + id)
	}
	return id, nil
}
