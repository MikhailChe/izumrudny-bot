package markup

import (
	"mikhailche/botcomod/services"
	"mikhailche/botcomod/tracer"

	tele "gopkg.in/telebot.v3"
)

var (
	HelpMainMenuBtn  = Data("⬅️ Назад в главное меню", "help-main-menu")
	DistrictChatsBtn = Data("💬🏘 Чаты района", "district-chats")

	HelpfulPhonesBtn = Data("☎️ Телефоны", "phone-numbers")

	ResidentsBtn       = Data("🏡 Для резидентов", "authorized-section")
	IntercomCodeBtn    = Data("🔑 Код домофона", "intercom-code")
	VideoCamerasBtn    = Data("📽 Камеры видеонаблюдения", "internal-video-cameras")
	PMWithResidentsBtn = Data("💬 Чат с другими резидентами", "resident-pm")

	RegisterBtn         = Data("📒 Начать регистрацию", "registration")
	ContinueRegisterBtn = Data("📒 Продолжить регистрацию", "registration")

	ChatGroupAdminBtn = Data("⚙️ Для админов чатов", "chatgroupadmin")
)

func HelpMenuMarkup() *tele.ReplyMarkup {
	defer tracer.Trace("helpMenuMarkup")()
	return InlineMarkup(
		Row(DistrictChatsBtn),
		Row(HelpfulPhonesBtn),
		Row(ResidentsBtn),
		Row(Text("🟢 Без комунальных проблем")),
	)
}

func DynamicHelpMenuMarkup(ctx tele.Context, groupChats *services.GroupChatService) *tele.ReplyMarkup {
	defer tracer.Trace("DynamicHelpMenuMarkup")()
	var rows []tele.Row
	isAdminOfSomeChat := isAdminOfSomeManagedChatFn(groupChats)(ctx, ctx.Sender().ID)
	rows = append(
		rows,
		Row(DistrictChatsBtn),
		Row(HelpfulPhonesBtn),
		Row(ResidentsBtn),
		Row(Text("🟢 Без комунальных проблем")),
	)
	if isAdminOfSomeChat {
		rows = append(rows, Row(ChatGroupAdminBtn))
	}
	return InlineMarkup(rows...)
}

var isAdminOfSomeManagedChatFnCache func(ctx tele.Context, userID int64) bool

func isAdminOfSomeManagedChatFn(groupChats *services.GroupChatService) func(ctx tele.Context, userID int64) bool {
	if isAdminOfSomeManagedChatFnCache != nil {
		return isAdminOfSomeManagedChatFnCache
	}
	byUserIDCache := make(map[int64]bool)
	isAdminOfSomeManagedChatFnCache = func(ctx tele.Context, userID int64) bool {
		if answer, inCache := byUserIDCache[userID]; inCache {
			return answer
		}
		byUserIDCache[userID] = func(userID int64) bool {
			api := ctx.Bot()
			chats := groupChats.GroupChats()
			for _, chat := range chats {
				chatID := chat.TelegramChatID
				if chatID == 0 {
					continue
				}
				chatById, err := api.ChatByID(chatID)
				if err != nil {
					continue
				}
				admins, err := api.AdminsOf(chatById)
				if err != nil {
					continue
				}
				for _, admin := range admins {
					if userID == admin.User.ID {
						return true
					}
				}
			}
			return false
		}(userID)
		return byUserIDCache[userID]
	}
	return isAdminOfSomeManagedChatFnCache
}
