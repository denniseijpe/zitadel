package session

import (
	"context"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/zitadel/zitadel/internal/api/authz"
	"github.com/zitadel/zitadel/internal/api/grpc/object/v2"
	"github.com/zitadel/zitadel/internal/command"
	"github.com/zitadel/zitadel/internal/domain"
	caos_errs "github.com/zitadel/zitadel/internal/errors"
	"github.com/zitadel/zitadel/internal/query"
	session "github.com/zitadel/zitadel/pkg/grpc/session/v2alpha"
)

func (s *Server) GetSession(ctx context.Context, req *session.GetSessionRequest) (*session.GetSessionResponse, error) {
	res, err := s.query.SessionByID(ctx, true, req.GetSessionId(), req.GetSessionToken())
	if err != nil {
		return nil, err
	}
	return &session.GetSessionResponse{
		Session: sessionToPb(res),
	}, nil
}

func (s *Server) ListSessions(ctx context.Context, req *session.ListSessionsRequest) (*session.ListSessionsResponse, error) {
	queries, err := listSessionsRequestToQuery(ctx, req)
	if err != nil {
		return nil, err
	}
	sessions, err := s.query.SearchSessions(ctx, queries)
	if err != nil {
		return nil, err
	}
	return &session.ListSessionsResponse{
		Details:  object.ToListDetails(sessions.SearchResponse),
		Sessions: sessionsToPb(sessions.Sessions),
	}, nil
}

func (s *Server) CreateSession(ctx context.Context, req *session.CreateSessionRequest) (*session.CreateSessionResponse, error) {
	checks, metadata, err := s.createSessionRequestToCommand(ctx, req)
	if err != nil {
		return nil, err
	}
	challengeResponse, cmds := s.challengesToCommand(req.GetChallenges(), checks)

	set, err := s.command.CreateSession(ctx, cmds, metadata)
	if err != nil {
		return nil, err
	}
	return &session.CreateSessionResponse{
		Details:      object.DomainToDetailsPb(set.ObjectDetails),
		SessionId:    set.ID,
		SessionToken: set.NewToken,
		Challenges:   challengeResponse,
	}, nil
}

func (s *Server) SetSession(ctx context.Context, req *session.SetSessionRequest) (*session.SetSessionResponse, error) {
	checks, err := s.setSessionRequestToCommand(ctx, req)
	if err != nil {
		return nil, err
	}
	challengeResponse, cmds := s.challengesToCommand(req.GetChallenges(), checks)

	set, err := s.command.UpdateSession(ctx, req.GetSessionId(), req.GetSessionToken(), cmds, req.GetMetadata())
	if err != nil {
		return nil, err
	}
	// if there's no new token, just return the current
	if set.NewToken == "" {
		set.NewToken = req.GetSessionToken()
	}
	return &session.SetSessionResponse{
		Details:      object.DomainToDetailsPb(set.ObjectDetails),
		SessionToken: set.NewToken,
		Challenges:   challengeResponse,
	}, nil
}

func (s *Server) DeleteSession(ctx context.Context, req *session.DeleteSessionRequest) (*session.DeleteSessionResponse, error) {
	details, err := s.command.TerminateSession(ctx, req.GetSessionId(), req.GetSessionToken())
	if err != nil {
		return nil, err
	}
	return &session.DeleteSessionResponse{
		Details: object.DomainToDetailsPb(details),
	}, nil
}

func sessionsToPb(sessions []*query.Session) []*session.Session {
	s := make([]*session.Session, len(sessions))
	for i, session := range sessions {
		s[i] = sessionToPb(session)
	}
	return s
}

func sessionToPb(s *query.Session) *session.Session {
	return &session.Session{
		Id:           s.ID,
		CreationDate: timestamppb.New(s.CreationDate),
		ChangeDate:   timestamppb.New(s.ChangeDate),
		Sequence:     s.Sequence,
		Factors:      factorsToPb(s),
		Metadata:     s.Metadata,
	}
}

func factorsToPb(s *query.Session) *session.Factors {
	user := userFactorToPb(s.UserFactor)
	if user == nil {
		return nil
	}
	return &session.Factors{
		User:     user,
		Password: passwordFactorToPb(s.PasswordFactor),
		WebAuthN: webAuthNFactorToPb(s.WebAuthNFactor),
		Intent:   intentFactorToPb(s.IntentFactor),
	}
}

func passwordFactorToPb(factor query.SessionPasswordFactor) *session.PasswordFactor {
	if factor.PasswordCheckedAt.IsZero() {
		return nil
	}
	return &session.PasswordFactor{
		VerifiedAt: timestamppb.New(factor.PasswordCheckedAt),
	}
}

func intentFactorToPb(factor query.SessionIntentFactor) *session.IntentFactor {
	if factor.IntentCheckedAt.IsZero() {
		return nil
	}
	return &session.IntentFactor{
		VerifiedAt: timestamppb.New(factor.IntentCheckedAt),
	}
}

func webAuthNFactorToPb(factor query.SessionWebAuthNFactor) *session.WebAuthNFactor {
	if factor.WebAuthNCheckedAt.IsZero() {
		return nil
	}
	return &session.WebAuthNFactor{
		VerifiedAt:   timestamppb.New(factor.WebAuthNCheckedAt),
		UserVerified: factor.UserVerified,
	}
}

func userFactorToPb(factor query.SessionUserFactor) *session.UserFactor {
	if factor.UserID == "" || factor.UserCheckedAt.IsZero() {
		return nil
	}
	return &session.UserFactor{
		VerifiedAt:     timestamppb.New(factor.UserCheckedAt),
		Id:             factor.UserID,
		LoginName:      factor.LoginName,
		DisplayName:    factor.DisplayName,
		OrganisationId: factor.ResourceOwner,
	}
}

func listSessionsRequestToQuery(ctx context.Context, req *session.ListSessionsRequest) (*query.SessionsSearchQueries, error) {
	offset, limit, asc := object.ListQueryToQuery(req.Query)
	queries, err := sessionQueriesToQuery(ctx, req.GetQueries())
	if err != nil {
		return nil, err
	}
	return &query.SessionsSearchQueries{
		SearchRequest: query.SearchRequest{
			Offset: offset,
			Limit:  limit,
			Asc:    asc,
		},
		Queries: queries,
	}, nil
}

func sessionQueriesToQuery(ctx context.Context, queries []*session.SearchQuery) (_ []query.SearchQuery, err error) {
	q := make([]query.SearchQuery, len(queries)+1)
	for i, query := range queries {
		q[i], err = sessionQueryToQuery(query)
		if err != nil {
			return nil, err
		}
	}
	creatorQuery, err := query.NewSessionCreatorSearchQuery(authz.GetCtxData(ctx).UserID)
	if err != nil {
		return nil, err
	}
	q[len(queries)] = creatorQuery
	return q, nil
}

func sessionQueryToQuery(query *session.SearchQuery) (query.SearchQuery, error) {
	switch q := query.Query.(type) {
	case *session.SearchQuery_IdsQuery:
		return idsQueryToQuery(q.IdsQuery)
	default:
		return nil, caos_errs.ThrowInvalidArgument(nil, "GRPC-Sfefs", "List.Query.Invalid")
	}
}

func idsQueryToQuery(q *session.IDsQuery) (query.SearchQuery, error) {
	return query.NewSessionIDsSearchQuery(q.Ids)
}

func (s *Server) createSessionRequestToCommand(ctx context.Context, req *session.CreateSessionRequest) ([]command.SessionCommand, map[string][]byte, error) {
	checks, err := s.checksToCommand(ctx, req.Checks)
	if err != nil {
		return nil, nil, err
	}
	return checks, req.GetMetadata(), nil
}

func (s *Server) setSessionRequestToCommand(ctx context.Context, req *session.SetSessionRequest) ([]command.SessionCommand, error) {
	checks, err := s.checksToCommand(ctx, req.Checks)
	if err != nil {
		return nil, err
	}
	return checks, nil
}

func (s *Server) checksToCommand(ctx context.Context, checks *session.Checks) ([]command.SessionCommand, error) {
	checkUser, err := userCheck(checks.GetUser())
	if err != nil {
		return nil, err
	}
	sessionChecks := make([]command.SessionCommand, 0, 3)
	if checkUser != nil {
		user, err := checkUser.search(ctx, s.query)
		if err != nil {
			return nil, err
		}
		sessionChecks = append(sessionChecks, command.CheckUser(user.ID))
	}
	if password := checks.GetPassword(); password != nil {
		sessionChecks = append(sessionChecks, command.CheckPassword(password.GetPassword()))
	}
	if intent := checks.GetIntent(); intent != nil {
		sessionChecks = append(sessionChecks, command.CheckIntent(intent.GetIntentId(), intent.GetToken()))
	}
	if passkey := checks.GetWebAuthN(); passkey != nil {
		sessionChecks = append(sessionChecks, s.command.CheckWebAuthN(passkey.GetCredentialAssertionData()))
	}

	return sessionChecks, nil
}

func (s *Server) challengesToCommand(challenges *session.RequestChallenges, cmds []command.SessionCommand) (*session.Challenges, []command.SessionCommand) {
	if challenges == nil {
		return nil, cmds
	}
	resp := new(session.Challenges)
	if req := challenges.GetWebAuthN(); req != nil {
		challenge, cmd := s.createWebAuthNChallengeCommand(req)
		resp.WebAuthN = challenge
		cmds = append(cmds, cmd)
	}
	return resp, cmds
}

func (s *Server) createWebAuthNChallengeCommand(req *session.RequestChallenges_WebAuthN) (*session.Challenges_WebAuthN, command.SessionCommand) {
	challenge := &session.Challenges_WebAuthN{
		PublicKeyCredentialRequestOptions: new(structpb.Struct),
	}
	userVerification := userVerificationRequirementToDomain(req.GetUserVerificationRequirement())
	return challenge, s.command.CreateWebAuthNChallenge(userVerification, req.GetDomain(), challenge.PublicKeyCredentialRequestOptions)
}

func userVerificationRequirementToDomain(req session.UserVerificationRequirement) domain.UserVerificationRequirement {
	switch req {
	case session.UserVerificationRequirement_USER_VERIFICATION_REQUIREMENT_UNSPECIFIED:
		return domain.UserVerificationRequirementUnspecified
	case session.UserVerificationRequirement_USER_VERIFICATION_REQUIREMENT_REQUIRED:
		return domain.UserVerificationRequirementRequired
	case session.UserVerificationRequirement_USER_VERIFICATION_REQUIREMENT_PREFERRED:
		return domain.UserVerificationRequirementPreferred
	case session.UserVerificationRequirement_USER_VERIFICATION_REQUIREMENT_DISCOURAGED:
		return domain.UserVerificationRequirementDiscouraged
	default:
		return domain.UserVerificationRequirementUnspecified
	}
}

func userCheck(user *session.CheckUser) (userSearch, error) {
	if user == nil {
		return nil, nil
	}
	switch s := user.GetSearch().(type) {
	case *session.CheckUser_UserId:
		return userByID(s.UserId), nil
	case *session.CheckUser_LoginName:
		return userByLoginName(s.LoginName)
	default:
		return nil, caos_errs.ThrowUnimplementedf(nil, "SESSION-d3b4g0", "user search %T not implemented", s)
	}
}

type userSearch interface {
	search(ctx context.Context, q *query.Queries) (*query.User, error)
}

func userByID(userID string) userSearch {
	return userSearchByID{userID}
}

func userByLoginName(loginName string) (userSearch, error) {
	loginNameQuery, err := query.NewUserLoginNamesSearchQuery(loginName)
	if err != nil {
		return nil, err
	}
	return userSearchByLoginName{loginNameQuery}, nil
}

type userSearchByID struct {
	id string
}

func (u userSearchByID) search(ctx context.Context, q *query.Queries) (*query.User, error) {
	return q.GetUserByID(ctx, true, u.id, false)
}

type userSearchByLoginName struct {
	loginNameQuery query.SearchQuery
}

func (u userSearchByLoginName) search(ctx context.Context, q *query.Queries) (*query.User, error) {
	return q.GetUser(ctx, true, false, u.loginNameQuery)
}
