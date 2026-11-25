package bootstrap

import (
	pb "collaboration/contracts/internalv1"
	"collaboration/document/internal/client"
	"collaboration/document/internal/handler"
	document "collaboration/document/internal/handler/document"
	logic "collaboration/document/internal/logic/document"
	"collaboration/document/internal/server"
	"collaboration/document/internal/svc"
	"collaboration/document/internal/worker"
	"collaboration/platform"
	"context"
	"errors"
	"github.com/zeromicro/go-zero/rest"
)

// RegisterDocument集中组合依赖与生命周期；业务资格、事务、SQL和运输细节分别留在所属层。
func RegisterDocument(rpc *server.Document) func(*rest.Server, *platform.Store, platform.Config) error {
	return func(httpServer *rest.Server, store *platform.Store, cfg platform.Config) error {
		outputs, err := svc.NewOutputs(store, cfg)
		if err != nil {
			return err
		}
		outputHTTP := document.NewOutputs(outputs)
		fences := svc.NewFences(store, cfg, outputs)
		authorizations := svc.NewAuthorizations(store, fences, outputs)
		store.ReadyExtra = outputs.Ready
		accountPeer, err := platform.DialPeer(cfg, "account")
		if err != nil {
			return err
		}
		// 组合失败关闭已创建的RPC连接；成功后由共用恢复worker的退出负责释放。
		assembled := false
		defer func() {
			if !assembled {
				accountPeer.Close()
			}
		}()
		searchPeer, err := platform.DialPeer(cfg, "search")
		if err != nil {
			return err
		}
		// Doc以自己的真实mTLS角色调用Search；连接装配失败关闭，成功随store退出释放。
		// 不把Search数据库/ES凭据带入Doc，也不在SQL事务里建立跨服务连接。
		defer func() {
			if !assembled {
				searchPeer.Close()
			}
		}()
		accountHTTP, err := platform.HTTPPeer(cfg, "account")
		if err != nil {
			return err
		}
		collaborationHTTP, err := platform.HTTPPeer(cfg, "collaboration")
		if err != nil {
			return err
		}
		departures := svc.NewDepartures(store, cfg, accountHTTP)
		accountClosure := client.NewAccountClosure(cfg.Peers["accountHttp"], accountHTTP)
		retirements := logic.NewAccountRetirements(store.DB, accountClosure)
		documentCleaner := client.NewAccountDocumentCleanup(cfg.Peers["collaborationHttp"], collaborationHTTP)
		retentions, err := svc.NewAccountRetentions(store.DB, accountClosure, documentCleaner)
		if err != nil {
			return err
		}
		organizationClosures, err := svc.NewOrganizationClosures(store.DB, client.NewOrganizationCloseAuthority(cfg.Peers["accountHttp"], accountHTTP), documentCleaner)
		if err != nil {
			return err
		}
		identity := svc.NewIdentity(pb.NewIdentityInternalClient(accountPeer), cfg.Peers["accountHttp"], accountHTTP)
		access := svc.NewAccess(store.DB, identity)
		authentication := document.NewIdentity(identity)
		requests := document.NewTransactions(svc.NewTransactions(store, identity, outputs), outputHTTP)
		pagination := svc.NewPagination(cfg)
		publicReading, err := svc.NewPublicReading(store, identity, access, outputs, pagination)
		if err != nil {
			return err
		}
		searchPages, err := svc.NewSearchPages(store, cfg, identity, access, outputs, pb.NewSearchInternalClient(searchPeer))
		if err != nil {
			return err
		}
		directoryClient := svc.NewDirectory(cfg, accountHTTP)
		tokens := client.NewInvitationTokens(cfg.KeyFile)
		editSessions := svc.NewEditSessions(store, cfg, access, tokens)
		submissions, err := svc.NewSubmissions(store, cfg, identity, access, outputs)
		if err != nil {
			return err
		}
		reviews, err := svc.NewReviews(store, access, pagination, directoryClient)
		if err != nil {
			return err
		}
		contentVersions, err := svc.NewContentVersions(store, access)
		if err != nil {
			return err
		}
		resources, err := svc.NewResources(store, cfg, access)
		if err != nil {
			return err
		}
		if err = resources.InitializeQuotas(store.Context); err != nil {
			return err
		}
		personalTransfers := svc.NewPersonalTransfers(store, identity, access, pagination, fences, client.NewPersonalTransferStorage(cfg.Peers["collaborationHttp"], collaborationHTTP), resources.SpaceQuotaLimit(), outputs.Self().ID)
		if !personalTransfers.Ready() {
			return errors.New("PERSONAL_TRANSFER_DEPENDENCY_UNAVAILABLE")
		}
		exports, err := svc.NewExports(store, cfg, identity, access, publicReading, outputs, resources)
		if err != nil {
			return err
		}
		merges, err := svc.NewMerges(store, cfg, identity, access, outputs)
		if err != nil {
			return err
		}
		controls, err := svc.NewMainControls(store, cfg, identity, access, outputs)
		if err != nil {
			return err
		}
		publications, err := svc.NewPublications(store, cfg, identity, access, outputs)
		if err != nil {
			return err
		}
		mainReadViews, err := svc.NewMainReadViews(store, cfg, identity)
		if err != nil {
			return err
		}
		readings, err := svc.NewReadings(access, mainReadViews)
		if err != nil {
			return err
		}
		pulls, err := svc.NewPullMain(store, cfg, identity, access, outputs)
		if err != nil {
			return err
		}
		structurePreviews, err := svc.NewStructurePreviews(store, cfg, identity, access, outputs)
		if err != nil {
			return err
		}
		draftControls, err := svc.NewDraftControls(store, cfg, identity, access, outputs)
		if err != nil {
			return err
		}
		reverseChanges, err := svc.NewReverseChanges(store, cfg, identity, access, outputs)
		if err != nil {
			return err
		}
		resolutions, err := svc.NewResolutions(store, cfg, identity, access, outputs)
		if err != nil {
			return err
		}
		store.ManagedTasks = []string{"SUBMISSION_RECOVER", "CAPTURE_FINALIZE", "REVIEW_NOTIFICATION", "REVIEW_ACTION_NOTIFICATION", "MERGE_RECOVER", "MAIN_RECONCILE", "MERGE_NOTIFICATION", "MAIN_CONTROL_RECOVER", "MAIN_CONTROL_RECONCILE", "PUBLICATION_RECOVER", "PULL_MAIN_RECOVER", "PULL_MAIN_RECONCILE", "STRUCTURE_PREVIEW_RECOVER", "DISCARD_PREVIEW_RECOVER", "DISCARD_DRAFT_RECOVER", "DISCARD_DRAFT_RECONCILE", "REVERSE_RECOVER", "REVERSE_RECONCILE", "RESOLUTION_RECOVER", "RESOLUTION_RECONCILE"}
		comments := logic.NewComments(store.DB, access, reviews, pagination)
		notificationRecipients := client.NewNotificationRecipients(cfg.Peers["accountHttp"], accountHTTP)
		notificationDelivery := logic.NewNotificationDelivery(store.DB, comments, notificationRecipients)
		store.ManagedTasks = append(store.ManagedTasks, "COMMENT_NOTIFICATION", "RESPONSIBLE_DEPARTURE_NOTIFICATION")
		store.ReadyExtra = func(ctx context.Context) error {
			if err := organizationClosures.Ready(ctx); err != nil {
				return err
			}
			if err := notificationDelivery.Ready(ctx); err != nil {
				return err
			}
			if err := outputs.Ready(ctx); err != nil {
				return err
			}
			if err := submissions.Ready(ctx); err != nil {
				return err
			}
			if err := reviews.Ready(ctx); err != nil {
				return err
			}
			if err := merges.Ready(ctx); err != nil {
				return err
			}
			if err := controls.Ready(ctx); err != nil {
				return err
			}
			if err := publications.Ready(ctx); err != nil {
				return err
			}
			if err := pulls.Ready(ctx); err != nil {
				return err
			}
			if err := structurePreviews.Ready(ctx); err != nil {
				return err
			}
			if err := draftControls.Ready(ctx); err != nil {
				return err
			}
			if err := reverseChanges.Ready(ctx); err != nil {
				return err
			}
			return resolutions.Ready(ctx)
		}
		sharing := document.NewSharing(authentication, requests, svc.NewSharing(store, cfg, identity, access, pagination, tokens, directoryClient))
		previewHTTP := document.NewStructurePreviews(authentication, requests, structurePreviews, sharing.OperationProgress)
		reverseHTTP := document.NewReverseChanges(authentication, requests, reverseChanges, previewHTTP.Progress)
		resolutionHTTP := document.NewConflictResolutions(authentication, requests, reviews, resolutions, reverseHTTP.Progress)
		draftHTTP := document.NewDraftControls(authentication, requests, draftControls, resolutionHTTP.Progress)
		pullHTTP := document.NewPullMain(authentication, requests, pulls, draftHTTP.Progress)
		controlHTTP := document.NewMainControls(authentication, requests, controls, pullHTTP.Progress)
		publicationHTTP := document.NewPublications(authentication, requests, publications, controlHTTP.Progress)
		mergeHTTP := document.NewMerges(authentication, requests, reviews, merges, publicationHTTP.Progress)
		knowledgeBases := svc.NewKnowledgeBases(store, identity, access)
		trash := logic.NewTrash(store.DB, identity, access, pagination)
		purges := svc.NewPurges(store, trash, documentCleaner, outputs.Self().ID)
		retainedGovernance := svc.NewRetainedGovernance(store, identity, documentCleaner, resources, exports, outputs.Self().ID)
		retainedHTTP := document.NewRetainedGovernance(authentication, requests, retainedGovernance, mergeHTTP.Progress)
		retirementHTTP := document.NewAccountRetirements(retirements, retentions)
		retirementHTTP.SetPurges(purges)
		retirementHTTP.SetRetainedGovernance(retainedGovernance)
		store.ManagedTasks = append(store.ManagedTasks, "PURGE_RECOVER", "RETAINED_GOVERNANCE_RECOVER", "PERSONAL_TRANSFER_RECOVER")

		// 平台在这个回调成功后才开放HTTP/RPC；所有适配器复用唯一已装配用例。
		draftResources := svc.NewDraftResources(store, cfg, collaborationHTTP)
		rpc.Bind(server.NewFence(fences), server.NewEditSessions(editSessions), server.NewDraftResources(draftResources), server.NewOperationResults(submissions, merges, controls, pulls, structurePreviews, draftControls, reverseChanges, resolutions, publications), server.NewPublicSnapshot(publicReading))
		rpc.BindLifecycle(logic.NewLifecyclePlans(store.DB, accountClosure, retirements, retentions))
		rpc.BindCandidateCleanup(logic.NewCandidateCleanup(store.DB))
		handler.RegisterDocument(httpServer, handler.Routes{
			DraftBodyCapacity:    document.NewDraftBodyCapacity(draftResources),
			Outputs:              outputHTTP,
			Departures:           document.NewDepartures(departures),
			AccountRetirements:   retirementHTTP,
			OrganizationClosures: document.NewOrganizationClosures(organizationClosures),
			Sharing:              sharing,
			Authorizations:       document.NewAuthorizations(authentication, requests, svc.NewAuthorizationChanges(store, identity, access), authorizations),
			Management:           document.NewManagement(authentication, requests, svc.NewManagement(store, identity, access)),
			Trash:                document.NewTrash(authentication, requests, trash),
			Purges:               document.NewPurges(authentication, requests, purges),
			RetainedGovernance:   retainedHTTP,
			PersonalTransfers:    document.NewPersonalTransfers(authentication, requests, personalTransfers),
			Exports:              document.NewExports(authentication, outputHTTP, exports, cfg.Origin),
			KnowledgeBases:       document.NewKnowledgeBases(authentication, requests, knowledgeBases),
			Directory:            document.NewDirectory(authentication, requests, logic.NewDirectory(store.DB, identity, access, knowledgeBases, pagination, directoryClient)),
			Visibilities:         document.NewVisibilities(authentication, requests, svc.NewVisibilities(knowledgeBases, access), authorizations),
			Documents:            document.NewDocuments(authentication, requests, svc.NewDocuments(store, identity, access, resources.SpaceQuotaLimit())),
			Readings:             document.NewReadings(authentication, requests, readings),
			ContentVersions:      document.NewContentVersions(authentication, requests, contentVersions),
			Resources:            document.NewResources(authentication, requests, resources, logic.NewResourceReading(resources, publicReading, reviews, outputs)),
			Comments:             document.NewComments(authentication, requests, comments),
			Notifications:        document.NewNotifications(authentication, requests, logic.NewNotifications(store.DB, access, comments, pagination, notificationRecipients)),
			Spaces:               document.NewSpaces(authentication, requests, svc.NewSpaces(identity, directoryClient, pagination)),
			EditableUnits:        document.NewEditableUnits(authentication, requests, logic.NewEditableUnits(access, pagination)),
			EditSessions:         document.NewEditSessions(authentication, requests, editSessions),
			Submissions:          document.NewSubmissions(authentication, requests, submissions, retainedHTTP.Progress),
			Reviews:              document.NewReviews(authentication, requests, reviews),
			Merges:               mergeHTTP,
			MainControls:         controlHTTP,
			Publications:         publicationHTTP,
			PublicReading:        document.NewPublicReading(authentication, requests, outputHTTP, publicReading),
			SearchPages:          document.NewSearchPages(authentication, outputHTTP, searchPages),
			MainSources:          document.NewMainSources(merges, controls, mainReadViews, publications),
			PullMain:             pullHTTP,
			DraftControls:        draftHTTP,
			ReverseChanges:       reverseHTTP,
			ConflictResolutions:  resolutionHTTP,
			DraftSources:         document.NewDraftSources(pulls, draftControls, reverseChanges, resolutions),
			StructurePreviews:    previewHTTP,
		})
		assembled = true
		go worker.RunSubmissions(store.Context, submissions)
		go worker.RunMerges(store.Context, merges)
		go worker.RunMainControls(store.Context, controls)
		go worker.RunPublications(store.Context, publications)
		go worker.RunResources(store.Context, resources)
		go worker.RunExports(store.Context, exports)
		go worker.RunPurges(store.Context, purges)
		go worker.RunRetainedGovernance(store.Context, retainedGovernance)
		go worker.RunPersonalTransfers(store.Context, personalTransfers)
		go worker.RunNotifications(store.Context, notificationDelivery)
		go worker.RunAccountNotifications(store.Context, logic.NewAccountNotificationImport(store.DB, notificationRecipients))
		go worker.RunPullMain(store.Context, pulls)
		go worker.RunStructurePreviews(store.Context, structurePreviews)
		go worker.RunDraftControls(store.Context, draftControls)
		go worker.RunReverseChanges(store.Context, reverseChanges)
		go worker.RunResolutions(store.Context, resolutions)
		go func() {
			defer accountPeer.Close()
			defer searchPeer.Close()
			worker.RunAuthorizations(store.Context, authorizations, departures.Recover)
		}()
		return nil
	}
}
