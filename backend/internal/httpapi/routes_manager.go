package httpapi

// registerManagerRoutes: 5B routes (characteristics, departments, invitations and the
// /api/manage/assessments list, PATCH, visibility, expiry, delete). 6B owns creation
// and the full-definition read in assessments_create.go (registerAssessmentCreateRoutes).
// Every manager route uses routeOpts{roles: rolesManager}.
func (s *Server) registerManagerRoutes() {
	mgr := routeOpts{roles: rolesManager}

	s.protected("GET /api/characteristics", s.characteristicsList, mgr)
	s.protected("POST /api/characteristics", s.characteristicsCreate, mgr)
	s.protected("PUT /api/characteristics/{id}", s.characteristicsRename, mgr)
	s.protected("DELETE /api/characteristics/{id}", s.characteristicsDelete, mgr)

	s.protected("GET /api/departments", s.departmentsList, mgr)
	s.protected("POST /api/departments", s.departmentsCreate, mgr)
	s.protected("PUT /api/departments/{id}", s.departmentsRename, mgr)
	s.protected("DELETE /api/departments/{id}", s.departmentsDelete, mgr)
	s.protected("POST /api/departments/{id}/activate", s.departmentsActivate, mgr)

	s.protected("GET /api/invitations", s.invitationsList, mgr)
	s.protected("POST /api/invitations", s.invitationsCreate, mgr)
	s.protected("PATCH /api/invitations/{id}", s.invitationsPatch, mgr)
	s.protected("POST /api/invitations/{id}/revoke", s.invitationsRevoke, mgr)
	s.protected("DELETE /api/invitations/{id}", s.invitationsDelete, mgr)
	s.protected("GET /api/invitations/{id}/registrations", s.invitationsRegistrations, mgr)

	s.protected("GET /api/manage/assessments", s.manageAssessmentsList, mgr)
	s.protected("GET /api/manage/assessments/{id}", s.manageAssessmentGet, mgr)
	s.protected("PATCH /api/manage/assessments/{id}", s.manageAssessmentsPatch, mgr)
	s.protected("PUT /api/manage/assessments/{id}/visibility", s.manageAssessmentsVisibility, mgr)
	s.protected("PUT /api/manage/assessments/{id}/expiry", s.manageAssessmentsExpiry, mgr)
	s.protected("DELETE /api/manage/assessments/{id}", s.manageAssessmentsDelete, mgr)

	s.registerAssessmentCreateRoutes()
}
