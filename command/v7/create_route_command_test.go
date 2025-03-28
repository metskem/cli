package v7_test

import (
	"errors"
	"fmt"
	"strconv"

	"code.cloudfoundry.org/cli/actor/actionerror"
	"code.cloudfoundry.org/cli/actor/v7action"
	"code.cloudfoundry.org/cli/api/cloudcontroller/ccversion"
	"code.cloudfoundry.org/cli/command/commandfakes"
	"code.cloudfoundry.org/cli/command/flag"
	. "code.cloudfoundry.org/cli/command/v7"
	"code.cloudfoundry.org/cli/command/v7/v7fakes"
	"code.cloudfoundry.org/cli/resources"
	"code.cloudfoundry.org/cli/util/configv3"
	"code.cloudfoundry.org/cli/util/ui"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
)

var _ = Describe("create-route Command", func() {
	var (
		cmd             CreateRouteCommand
		testUI          *ui.UI
		fakeConfig      *commandfakes.FakeConfig
		fakeSharedActor *commandfakes.FakeSharedActor
		fakeActor       *v7fakes.FakeActor

		executeErr error

		binaryName      string
		domainName      string
		spaceName       string
		spaceGUID       string
		orgName         string
		hostname        string
		path            string
		port            int
		cmdOptions      []string
		options         map[string]*string
		cCAPIOldVersion string
	)

	BeforeEach(func() {
		testUI = ui.NewTestUI(nil, NewBuffer(), NewBuffer())
		fakeConfig = new(commandfakes.FakeConfig)
		fakeConfig.APIVersionReturns(ccversion.MinVersionPerRouteOpts)
		fakeSharedActor = new(commandfakes.FakeSharedActor)
		fakeActor = new(v7fakes.FakeActor)

		domainName = "example.com"
		spaceName = "space"
		spaceGUID = "space-guid"
		orgName = "org"
		hostname = ""
		path = ""
		port = 0

		cmdOptions = []string{"loadbalancing=least-connection"}
		lbLCVal := "least-connection"
		lbLeastConnections := &lbLCVal
		options = map[string]*string{"loadbalancing": lbLeastConnections}

		binaryName = "faceman"
		fakeConfig.BinaryNameReturns(binaryName)
	})

	JustBeforeEach(func() {
		cmd = CreateRouteCommand{
			RequiredArgs: flag.Domain{
				Domain: domainName,
			},
			Hostname: hostname,
			Path:     flag.V7RoutePath{Path: path},
			Port:     port,
			Options:  cmdOptions,
			BaseCommand: BaseCommand{
				UI:          testUI,
				Config:      fakeConfig,
				SharedActor: fakeSharedActor,
				Actor:       fakeActor,
			},
		}
		executeErr = cmd.Execute(nil)
	})

	When("the environment is not set up correctly", func() {
		BeforeEach(func() {
			fakeSharedActor.CheckTargetReturns(actionerror.NotLoggedInError{BinaryName: binaryName})
		})

		It("returns an error", func() {
			Expect(executeErr).To(MatchError(actionerror.NotLoggedInError{BinaryName: binaryName}))

			Expect(fakeSharedActor.CheckTargetCallCount()).To(Equal(1))
			checkTargetedOrg, checkTargetedSpace := fakeSharedActor.CheckTargetArgsForCall(0)
			Expect(checkTargetedOrg).To(BeTrue())
			Expect(checkTargetedSpace).To(BeTrue())
		})
	})

	When("the environment is setup correctly", func() {
		BeforeEach(func() {
			fakeActor.GetCurrentUserReturns(configv3.User{Name: "the-user"}, nil)
			fakeConfig.TargetedSpaceReturns(configv3.Space{
				Name: spaceName,
				GUID: spaceGUID,
			})
			fakeConfig.TargetedOrganizationReturns(configv3.Organization{
				Name: orgName,
				GUID: "some-org-guid",
			})

			fakeActor.CreateRouteReturns(resources.Route{
				URL: domainName,
			}, v7action.Warnings{"warnings-1", "warnings-2"}, nil)
		})

		It("prints text indicating it is creating a route", func() {
			Expect(executeErr).NotTo(HaveOccurred())
			Expect(testUI.Out).To(Say(`Creating route %s for org %s / space %s as the-user\.\.\.`, domainName, orgName, spaceName))
		})

		When("passing in a hostname", func() {
			BeforeEach(func() {
				hostname = "flan"
			})

			It("prints text indicating it is creating a route", func() {
				Expect(executeErr).NotTo(HaveOccurred())
				Expect(testUI.Out).To(Say(`Creating route %s\.%s for org %s / space %s as the-user\.\.\.`, hostname, domainName, orgName, spaceName))
			})
		})

		When("passing in a path", func() {
			BeforeEach(func() {
				path = "/lion"

				fakeActor.CreateRouteReturns(resources.Route{
					URL: domainName + path,
				}, v7action.Warnings{"warnings-1", "warnings-2"}, nil)
			})

			It("prints information about the created route ", func() {
				Expect(executeErr).NotTo(HaveOccurred())
				Expect(testUI.Out).To(Say(`Creating route %s%s for org %s / space %s as the-user\.\.\.`, domainName, path, orgName, spaceName))
				Expect(testUI.Out).To(Say("Route %s%s has been created.", domainName, path))
				Expect(testUI.Out).To(Say("OK"))
			})
		})

		When("creating the route errors", func() {
			BeforeEach(func() {
				fakeActor.CreateRouteReturns(resources.Route{}, v7action.Warnings{"warnings-1", "warnings-2"}, errors.New("err-create-route"))
			})

			It("returns an error and displays warnings", func() {
				Expect(executeErr).To(MatchError("err-create-route"))
				Expect(testUI.Err).To(Say("warnings-1"))
				Expect(testUI.Err).To(Say("warnings-2"))
			})
		})

		When("creating the route does not fail when the CC API version is too old for route options", func() {
			BeforeEach(func() {
				cCAPIOldVersion = strconv.Itoa(1)
				fakeConfig.APIVersionReturns(cCAPIOldVersion)
			})

			It("does create a route and gives a warning message", func() {
				Expect(executeErr).NotTo(HaveOccurred())
				Expect(fakeActor.CreateRouteCallCount()).To(Equal(1))
				Expect(testUI.Err).To(Say("Your CC API"))
				Expect(testUI.Err).To(Say("does not support per-route options"))
			})
		})

		When("creating the route fails when route options are specified incorrectly", func() {
			BeforeEach(func() {
				cmdOptions = []string{"loadbalancing"}
			})

			It("does not create a route and gives an error message", func() {
				Expect(executeErr).To(MatchError(actionerror.RouteOptionError{Name: "loadbalancing", DomainName: domainName, Path: path, Host: hostname}))
				Expect(fakeActor.CreateRouteCallCount()).To(Equal(0))
			})
		})

		When("creating the route is successful", func() {
			BeforeEach(func() {
				fakeActor.CreateRouteReturns(resources.Route{
					URL:     domainName,
					Options: options,
				}, v7action.Warnings{"warnings-1", "warnings-2"}, nil)
			})

			It("prints all warnings, text indicating creation completion, ok and then a tip", func() {
				Expect(executeErr).ToNot(HaveOccurred())
				Expect(testUI.Err).To(Say("warnings-1"))
				Expect(testUI.Err).To(Say("warnings-2"))
				Expect(testUI.Out).To(Say(`Route %s has been created.`, domainName))
				Expect(testUI.Out).To(Say("OK"))
			})

			When("in a version of CAPI that does not support options", func() {
				BeforeEach(func() {
					fakeActor.CreateRouteReturns(resources.Route{
						URL: domainName,
					}, v7action.Warnings{"warnings-1", "warnings-2"}, nil)
					cmdOptions = []string{}
					cCAPIOldVersion = strconv.Itoa(1)
					fakeConfig.APIVersionReturns(cCAPIOldVersion)
				})

				It("creates the route when no options are provided", func() {
					Expect(fakeActor.CreateRouteCallCount()).To(Equal(1))
					expectedSpaceGUID, expectedDomainName, expectedHostname, _, _, expectedOptions := fakeActor.CreateRouteArgsForCall(0)
					Expect(expectedSpaceGUID).To(Equal(spaceGUID))
					Expect(expectedDomainName).To(Equal(domainName))
					Expect(expectedHostname).To(Equal(hostname))
					Expect(expectedOptions).To(BeNil())
				})
			})

			When("passing in a hostname", func() {
				BeforeEach(func() {
					hostname = "flan"

					fakeActor.CreateRouteReturns(resources.Route{
						URL: hostname + "." + domainName,
					}, v7action.Warnings{"warnings-1", "warnings-2"}, nil)
				})

				It("prints all warnings, text indicating creation completion, ok and then a tip", func() {
					Expect(executeErr).ToNot(HaveOccurred())
					Expect(testUI.Err).To(Say("warnings-1"))
					Expect(testUI.Err).To(Say("warnings-2"))
					Expect(testUI.Out).To(Say(`Route %s\.%s has been created.`, hostname, domainName))
					Expect(testUI.Out).To(Say("OK"))
				})

				It("creates the route", func() {
					Expect(fakeActor.CreateRouteCallCount()).To(Equal(1))
					expectedSpaceGUID, expectedDomainName, expectedHostname, _, _, _ := fakeActor.CreateRouteArgsForCall(0)
					Expect(expectedSpaceGUID).To(Equal(spaceGUID))
					Expect(expectedDomainName).To(Equal(domainName))
					Expect(expectedHostname).To(Equal(hostname))
				})
			})

			When("passing in a port", func() {
				BeforeEach(func() {
					port = 1234

					fakeActor.CreateRouteReturns(resources.Route{
						URL: domainName + ":" + fmt.Sprintf("%d", port),
					}, v7action.Warnings{"warnings-1", "warnings-2"}, nil)
				})

				It("prints all warnings, text indicating creation completion, ok and then a tip", func() {
					Expect(executeErr).ToNot(HaveOccurred())
					Expect(testUI.Err).To(Say("warnings-1"))
					Expect(testUI.Err).To(Say("warnings-2"))
					Expect(testUI.Out).To(Say(`Route %s:%d has been created.`, domainName, port))
					Expect(testUI.Out).To(Say("OK"))
				})

				It("calls the actor with the correct arguments", func() {
					Expect(fakeActor.CreateRouteCallCount()).To(Equal(1))
					expectedSpaceGUID, expectedDomainName, expectedHostname, _, expectedPort, expectedOptions := fakeActor.CreateRouteArgsForCall(0)
					Expect(expectedSpaceGUID).To(Equal(spaceGUID))
					Expect(expectedDomainName).To(Equal(domainName))
					Expect(expectedHostname).To(Equal(hostname))
					Expect(expectedPort).To(Equal(port))
					Expect(expectedOptions).To(Equal(options))
				})
			})
		})

		When("the route already exists", func() {
			BeforeEach(func() {
				fakeActor.CreateRouteReturns(resources.Route{}, v7action.Warnings{"some-warning"}, actionerror.RouteAlreadyExistsError{Err: errors.New("api error for a route that already exists")})
			})

			It("displays all warnings, that the route already exists, and does not error", func() {
				Expect(executeErr).ToNot(HaveOccurred())

				Expect(testUI.Err).To(Say("some-warning"))
				Expect(testUI.Err).To(Say("api error for a route that already exists"))
				Expect(testUI.Out).To(Say(`Creating route %s for org %s / space %s as the-user\.\.\.`, domainName, orgName, spaceName))
				Expect(testUI.Out).To(Say("OK"))
			})
		})
	})
})
