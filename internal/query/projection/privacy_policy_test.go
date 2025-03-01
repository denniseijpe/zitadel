package projection

import (
	"testing"

	"github.com/zitadel/zitadel/internal/domain"
	"github.com/zitadel/zitadel/internal/errors"
	"github.com/zitadel/zitadel/internal/eventstore"
	"github.com/zitadel/zitadel/internal/eventstore/handler"
	"github.com/zitadel/zitadel/internal/eventstore/repository"
	"github.com/zitadel/zitadel/internal/repository/instance"
	"github.com/zitadel/zitadel/internal/repository/org"
)

func TestPrivacyPolicyProjection_reduces(t *testing.T) {
	type args struct {
		event func(t *testing.T) eventstore.Event
	}
	tests := []struct {
		name   string
		args   args
		reduce func(event eventstore.Event) (*handler.Statement, error)
		want   wantReduce
	}{
		{
			name: "org reduceAdded",
			args: args{
				event: getEvent(testEvent(
					repository.EventType(org.PrivacyPolicyAddedEventType),
					org.AggregateType,
					[]byte(`{
						"tosLink": "http://tos.link",
						"privacyLink": "http://privacy.link",
						"helpLink": "http://help.link",
						"supportEmail": "support@example.com"}`),
				), org.PrivacyPolicyAddedEventMapper),
			},
			reduce: (&privacyPolicyProjection{}).reduceAdded,
			want: wantReduce{
				aggregateType:    eventstore.AggregateType("org"),
				sequence:         15,
				previousSequence: 10,
				executer: &testExecuter{
					executions: []execution{
						{
							expectedStmt: "INSERT INTO projections.privacy_policies3 (creation_date, change_date, sequence, id, state, privacy_link, tos_link, help_link, support_email, is_default, resource_owner, instance_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)",
							expectedArgs: []interface{}{
								anyArg{},
								anyArg{},
								uint64(15),
								"agg-id",
								domain.PolicyStateActive,
								"http://privacy.link",
								"http://tos.link",
								"http://help.link",
								domain.EmailAddress("support@example.com"),
								false,
								"ro-id",
								"instance-id",
							},
						},
					},
				},
			},
		},
		{
			name:   "org reduceChanged",
			reduce: (&privacyPolicyProjection{}).reduceChanged,
			args: args{
				event: getEvent(testEvent(
					repository.EventType(org.PrivacyPolicyChangedEventType),
					org.AggregateType,
					[]byte(`{
						"tosLink": "http://tos.link",
						"privacyLink": "http://privacy.link",
						"helpLink": "http://help.link",
						"supportEmail": "support@example.com"}`),
				), org.PrivacyPolicyChangedEventMapper),
			},
			want: wantReduce{
				aggregateType:    eventstore.AggregateType("org"),
				sequence:         15,
				previousSequence: 10,
				executer: &testExecuter{
					executions: []execution{
						{
							expectedStmt: "UPDATE projections.privacy_policies3 SET (change_date, sequence, privacy_link, tos_link, help_link, support_email) = ($1, $2, $3, $4, $5, $6) WHERE (id = $7) AND (instance_id = $8)",
							expectedArgs: []interface{}{
								anyArg{},
								uint64(15),
								"http://privacy.link",
								"http://tos.link",
								"http://help.link",
								domain.EmailAddress("support@example.com"),
								"agg-id",
								"instance-id",
							},
						},
					},
				},
			},
		},
		{
			name:   "org reduceRemoved",
			reduce: (&privacyPolicyProjection{}).reduceRemoved,
			args: args{
				event: getEvent(testEvent(
					repository.EventType(org.PrivacyPolicyRemovedEventType),
					org.AggregateType,
					nil,
				), org.PrivacyPolicyRemovedEventMapper),
			},
			want: wantReduce{
				aggregateType:    eventstore.AggregateType("org"),
				sequence:         15,
				previousSequence: 10,
				executer: &testExecuter{
					executions: []execution{
						{
							expectedStmt: "DELETE FROM projections.privacy_policies3 WHERE (id = $1) AND (instance_id = $2)",
							expectedArgs: []interface{}{
								"agg-id",
								"instance-id",
							},
						},
					},
				},
			},
		}, {
			name: "instance reduceInstanceRemoved",
			args: args{
				event: getEvent(testEvent(
					repository.EventType(instance.InstanceRemovedEventType),
					instance.AggregateType,
					nil,
				), instance.InstanceRemovedEventMapper),
			},
			reduce: reduceInstanceRemovedHelper(PrivacyPolicyInstanceIDCol),
			want: wantReduce{
				aggregateType:    eventstore.AggregateType("instance"),
				sequence:         15,
				previousSequence: 10,
				executer: &testExecuter{
					executions: []execution{
						{
							expectedStmt: "DELETE FROM projections.privacy_policies3 WHERE (instance_id = $1)",
							expectedArgs: []interface{}{
								"agg-id",
							},
						},
					},
				},
			},
		},
		{
			name:   "instance reduceAdded",
			reduce: (&privacyPolicyProjection{}).reduceAdded,
			args: args{
				event: getEvent(testEvent(
					repository.EventType(instance.PrivacyPolicyAddedEventType),
					instance.AggregateType,
					[]byte(`{
						"tosLink": "http://tos.link",
						"privacyLink": "http://privacy.link",
						"helpLink": "http://help.link",
						"supportEmail": "support@example.com"}`),
				), instance.PrivacyPolicyAddedEventMapper),
			},
			want: wantReduce{
				aggregateType:    eventstore.AggregateType("instance"),
				sequence:         15,
				previousSequence: 10,
				executer: &testExecuter{
					executions: []execution{
						{
							expectedStmt: "INSERT INTO projections.privacy_policies3 (creation_date, change_date, sequence, id, state, privacy_link, tos_link, help_link, support_email, is_default, resource_owner, instance_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)",
							expectedArgs: []interface{}{
								anyArg{},
								anyArg{},
								uint64(15),
								"agg-id",
								domain.PolicyStateActive,
								"http://privacy.link",
								"http://tos.link",
								"http://help.link",
								domain.EmailAddress("support@example.com"),
								true,
								"ro-id",
								"instance-id",
							},
						},
					},
				},
			},
		},
		{
			name:   "instance reduceChanged",
			reduce: (&privacyPolicyProjection{}).reduceChanged,
			args: args{
				event: getEvent(testEvent(
					repository.EventType(instance.PrivacyPolicyChangedEventType),
					instance.AggregateType,
					[]byte(`{
						"tosLink": "http://tos.link",
						"privacyLink": "http://privacy.link",
						"helpLink": "http://help.link",
						"supportEmail": "support@example.com"}`),
				), instance.PrivacyPolicyChangedEventMapper),
			},
			want: wantReduce{
				aggregateType:    eventstore.AggregateType("instance"),
				sequence:         15,
				previousSequence: 10,
				executer: &testExecuter{
					executions: []execution{
						{
							expectedStmt: "UPDATE projections.privacy_policies3 SET (change_date, sequence, privacy_link, tos_link, help_link, support_email) = ($1, $2, $3, $4, $5, $6) WHERE (id = $7) AND (instance_id = $8)",
							expectedArgs: []interface{}{
								anyArg{},
								uint64(15),
								"http://privacy.link",
								"http://tos.link",
								"http://help.link",
								domain.EmailAddress("support@example.com"),
								"agg-id",
								"instance-id",
							},
						},
					},
				},
			},
		},
		{
			name:   "org.reduceOwnerRemoved",
			reduce: (&privacyPolicyProjection{}).reduceOwnerRemoved,
			args: args{
				event: getEvent(testEvent(
					repository.EventType(org.OrgRemovedEventType),
					org.AggregateType,
					nil,
				), org.OrgRemovedEventMapper),
			},
			want: wantReduce{
				aggregateType:    eventstore.AggregateType("org"),
				sequence:         15,
				previousSequence: 10,
				executer: &testExecuter{
					executions: []execution{
						{
							expectedStmt: "UPDATE projections.privacy_policies3 SET (change_date, sequence, owner_removed) = ($1, $2, $3) WHERE (instance_id = $4) AND (resource_owner = $5)",
							expectedArgs: []interface{}{
								anyArg{},
								uint64(15),
								true,
								"instance-id",
								"agg-id",
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := baseEvent(t)
			got, err := tt.reduce(event)
			if _, ok := err.(errors.InvalidArgument); !ok {
				t.Errorf("no wrong event mapping: %v, got: %v", err, got)
			}

			event = tt.args.event(t)
			got, err = tt.reduce(event)
			assertReduce(t, got, err, PrivacyPolicyTable, tt.want)
		})
	}
}
