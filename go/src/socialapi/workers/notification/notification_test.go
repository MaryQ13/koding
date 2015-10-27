package notification

import (
	"koding/db/mongodb/modelhelper"
	"socialapi/config"
	"socialapi/models"
	socialapimodels "socialapi/models"
	"socialapi/workers/common/tests"
	"testing"

	"github.com/koding/runner"
	. "github.com/smartystreets/goconvey/convey"
)

func TestCleanup(t *testing.T) {
	testData := []struct {
		definition string
		usernames  []string
		expected   []string
	}{
		{
			"should remove aliases",
			[]string{"team", "all"},
			[]string{"all"},
		},
		{
			"should return same usernames",
			[]string{"foo", "bar", "zaar"},
			[]string{"foo", "bar", "zaar"},
		},
		{
			"should remove duplicates",
			[]string{"admins", "admins", "ff"},
			[]string{"admins", "ff"},
		},
		{
			"should remove specific ones if have a general one",
			[]string{"admins", "admins", "team"},
			[]string{"all"},
		},
		{
			"should reduce to global alias",
			[]string{"team", "all", "group"},
			[]string{"all"},
		},
		{
			// some of the admins may not be in the channel
			"should keep channel and admins",
			[]string{"channel", "bar", "admins"},
			[]string{"channel", "bar", "admins"},
		},
	}

	for _, test := range testData {
		responses := cleanup(test.usernames)
		exists := false
		for _, response := range responses {
			for _, exc := range test.expected {
				if exc == response {
					exists = true
					break
				}
			}
		}

		if !exists {
			t.Fatalf("%s. expected: %+v, got: %+v", test.definition, responses)
		}

		if len(test.expected) != len(responses) {
			t.Fatalf("%s, %s. expected: %+v, got: %+v", test.definition, "expected lengths are not same", test.expected, responses)
		}
	}
}

func TestNormalizeUsernames(t *testing.T) {
	tests.WithRunner(t, func(r *runner.Runner) {

		appConfig := config.MustRead(r.Conf.Path)
		modelhelper.Initialize(appConfig.Mongo)
		defer modelhelper.Close()

		Convey("while normalizing the usernames to their original nicks", t, func() {
			adminAccount, groupChannel, _ := models.CreateRandomGroupDataWithChecks()
			account1 := models.CreateAccountInBothDbsWithCheck()
			account2 := models.CreateAccountInBothDbsWithCheck()
			account3 := models.CreateAccountInBothDbsWithCheck()

			_, err := groupChannel.AddParticipant(account1.Id)
			So(err, ShouldBeNil)
			_, err = groupChannel.AddParticipant(account2.Id)
			So(err, ShouldBeNil)
			_, err = groupChannel.AddParticipant(account3.Id)
			So(err, ShouldBeNil)

			topicChan := socialapimodels.CreateTypedGroupedChannelWithTest(account1.Id, socialapimodels.Channel_TYPE_TOPIC, groupChannel.GroupName)

			Convey("@all should return all the members of the team", func() {
				body := "hi @all i am really excited to join this team!"
				cm := models.CreateMessageWithBody(groupChannel.Id, adminAccount.Id, models.ChannelMessage_TYPE_POST, body)

				usernames, err := normalizeUsernames(cm, []string{"all"})
				So(err, ShouldBeNil)
				So(len(usernames), ShouldEqual, 3)

				Convey("poster should not be in the mention list", func() {
					So(socialapimodels.IsIn(adminAccount.Nick, usernames...), ShouldBeFalse)
				})
			})

			Convey("@team should return all the members of the team", func() {
				_, err := topicChan.AddParticipant(adminAccount.Id)
				So(err, ShouldBeNil)
				_, err = topicChan.AddParticipant(account2.Id)
				So(err, ShouldBeNil)
				_, err = topicChan.AddParticipant(account3.Id)
				So(err, ShouldBeNil)

				body := "hi @team i am really excited to join this chan!"
				cm := models.CreateMessageWithBody(topicChan.Id, adminAccount.Id, models.ChannelMessage_TYPE_POST, body)

				usernames, err := normalizeUsernames(cm, []string{"team"})
				So(err, ShouldBeNil)
				So(len(usernames), ShouldEqual, 3)
			})

			Convey("@all + any username should return all the members of the team", func() {
				body := "hi @all i am really excited to join this team! how are you @" + account3.Nick
				cm := models.CreateMessageWithBody(groupChannel.Id, adminAccount.Id, models.ChannelMessage_TYPE_POST, body)

				usernames, err := normalizeUsernames(cm, []string{"all", account3.Nick})
				So(err, ShouldBeNil)
				So(len(usernames), ShouldEqual, 3)
			})

			Convey("@channel should return all the members of the channel", func() {

				body := "hi @channel"
				cm := socialapimodels.CreateMessageWithBody(topicChan.Id, account1.Id, models.ChannelMessage_TYPE_POST, body)

				Convey("if channel doesnt have any members", func() {
					Convey("should return 0 username", func() {
						usernames, err := normalizeUsernames(cm, []string{"channel"})
						So(err, ShouldBeNil)
						So(len(usernames), ShouldEqual, 0)
					})
				})

				Convey("if channel have member", func() {
					Convey("should return them", func() {
						_, err := topicChan.AddParticipant(account2.Id)
						So(err, ShouldBeNil)

						usernames, err := normalizeUsernames(cm, []string{"channel"})
						So(err, ShouldBeNil)
						So(len(usernames), ShouldEqual, 1)
						So(usernames[0], ShouldEqual, account2.Nick)
					})
				})
			})

			Convey("@channel + @group should return all the members of the team", func() {
				body := "hi @channel i am glad that i joined @group"
				cm := socialapimodels.CreateMessageWithBody(topicChan.Id, account1.Id, models.ChannelMessage_TYPE_POST, body)

				usernames, err := normalizeUsernames(cm, []string{"channel", "group"})
				So(err, ShouldBeNil)
				So(len(usernames), ShouldEqual, 3)
			})
		})
	})
}
