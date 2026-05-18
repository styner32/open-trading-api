package domesticstock

import (
	"context"
	"net/http"

	"github.com/kis-open-api/go/internal/auth"
	"github.com/kis-open-api/go/internal/testhelpers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Quotation analysis", func() {
	It("fetches KOSPI investor daily flow by market", func() {
		client := auth.NewKIClient("app-key", "app-secret", "https://example.test", "test-agent")
		client.SetAuthToken("test-token")

		transport := testhelpers.NewMockTransport()
		DeferCleanup(func() {
			Expect(transport.Verify()).To(Succeed())
		})

		transport.New("https://example.test").
			Get("/uapi/domestic-stock/v1/quotations/inquire-investor-daily-by-market?FID_COND_MRKT_DIV_CODE=U&FID_INPUT_DATE_1=20260515&FID_INPUT_DATE_2=20260515&FID_INPUT_ISCD=0001&FID_INPUT_ISCD_1=KSP&FID_INPUT_ISCD_2=0001").
			MatchHeader("authorization", "Bearer test-token").
			MatchHeader("tr_id", investorDailyByMarketTRID).
			Reply(http.StatusOK).
			JSON(map[string]any{
				"rt_cd": "0", "msg_cd": "0", "msg1": "정상처리 되었습니다.",
				"output": []any{map[string]any{"frgn_ntby_tr_pbmn": "-4834200"}},
			})

		client.Client = &http.Client{Transport: transport}
		resp, err := NewService(client).InquireInvestorDailyByMarket(context.Background(), "20260515")
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.IsOK()).To(BeTrue())
	})
})
