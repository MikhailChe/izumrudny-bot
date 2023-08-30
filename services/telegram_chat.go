package telegram_chat

import "context"

func ErrorfOrNil(e error, format string, args ... any) error {
	if e == nil{
		return nil
	}
	if len(format) == 0 {
		return err
	}
	return fmt.Errorf(fmt.Sprintf(format, args...) + ": %w", e)
}

// CACHED MAPPING OF TELEGRAM CHAT TO SENDER

const upsert_cached_telegram_chat_to_sender_mapping_query = `
DECLARE $chat_id AS Int64;
DECLARE $user_id AS Int64;

UPSERT INTO telegram_chat_to_user
(chat_id, user_id)
VALUES
($chat_id, $user_id)
;
`

func UpsertTelegramChatToUserMapping(ydb *ydb.Driver) func(ctx context.Context, chat, user int64) error {
	return func(ctx contex.Context, chat, user int64) error {
		defer tracer.Trace("UpsertTelegramChatToUserMapping")()
		return ydb.Table().Do(ctx, func(ctx context.Context, sess table.Session) error {
			defer tracer.Trace("UpsertTelegramChatToUserMapping::Do")
			_, _, err := sess.Execute(
				ctx,
				table.DefaultTxContorl(),
				upsert_cached_telegram_chat_to_sender_mapping_query,
				table.NewQueryParameters(
					table.ValueParam("$user_id", types.Int64Value(user)),
					table.ValueParam("$chat_id", types.Int64Value(chat)),
				),
			)
			if err != nil {
				return fmt.Errorf("UPSERT INTO telegram_chat_to_user: %w", err)
			}
			return nil
		}, ydb.DefaultTxDefenition())
	}
}




// SELECT USER by USERNAME AND ID

type getUserOption = func(ctx context.Context)(txr Transaction, r result.Result, err error)

func (r *UserRepository) ByID(userID int64) func(ctx context.Context)(txr Transaction, r result.Result, err error){
	return func()(txr Transaction, r result.Result, err error){
		s.Execute(ctx, table.DefaultTxControl(),
`DECLARE $id AS Int64;
SELECT * FROM user WHERE id = $id LIMIT 1;`,
			table.NewQueryParameters(table.ValueParam("$id", types.Int64Value(userID))),
		)
	}
}


func (r *UserRepository) ByUsername(username string) func(ctx context.Context)(txr Transaction, r result.Result, err error){
	return func()(txr Transaction, r result.Result, err error){ s.Execute(ctx, table.DefaultTxControl(),
			`DECLARE $username AS Utf8;
SELECT * FROM user WHERE username = $username LIMIT 1;`,
			table.NewQueryParameters(table.ValueParam("$id", types.Int64Value(userID))),
		)
	}
}

func (r *UserRepository) ApplyEvents(ctx context.Context, user *User) error {
	defer tracer.Trace("UserRepository::ApplyEvents")
	if err := r.DB.Table().Do(ctx, func(ctx context.Context, s table.Session) error{
		_, res, err := s.Execute(ctx, table.DefaultTxContorl(),
`DECLARE $id AS Int64;
SELECT * FROM user_event WHERE user = $id ORDER BY user, timestamp, id;`,
			table.NewQueryParameters(table.ValueParam("$id", type.Int64Value(user.ID))),
		)
		if err != nil {
			return fmt.Errorf("SELECT user_event [id=%d]: %w", user.ID, err)
		}
		defer res.Close()
		if !res.NextResultSet(ctx) {
			return fmt.Errorf("не нашел result set для событий пользователя; невалидный запрос?")
		}
		for res.NextRow() {
			var event UserEventRecord
			if err := event.Scan(res); err!=nil{
				return fmt.Errorf("не смог события пользователя: %w", err)
			}
			r.log.Info("Применяю собятие", zap.Any("event", event))
			event.Event.Apply(user)
			user.Events = append(user.Events, event)
		}
		return ErrorOrNil(res.Err(), "ApplyEvents [id=%d]", user.ID)
	})
}

func (r *UserRepository) GetUser(ctx context.Context, userQueryExecutor getUserOption) (*User, error) {
	defer tracer.Trace("UserRepository::GetById")()
	var user User
	if err := r.DB.Table().Do(ctx, func(ctx context.Context, s table.Session) error {
		defer tracer.Trace("UserRepository::GetById::Do")()
		_, res, err := userQueryExecutor(ctx)
		if err != nil {
			return fmt.Errorf("select user [%d]: %w", userID, err)
		}
		defer tracer.Trace("UserRepository::GetById::DoUser")()
		defer res.Close()
		if !res.NextResultSet(ctx) {
			return fmt.Errorf("не нашел result set для пользователя")
		}
		if !res.NextRow() {
			return fmt.Errorf("пользователь не найден")
		}
		if err := user.Scan(res); err != nil {
			return fmt.Errorf("скан пользователя %v: %w", res, err)
		}
		return r.ApplyEvents(ctx, &user)
	}, table.WithIdempotent()); err != nil {
		return nil, err
	}
	return &user, nil
}


const user_by_username_query = `
DECLARE $username AS Utf8;
SELECT * FROM user WHERE username = $id LIMIT 1;
`

func SelectUserByUsername(ydb *ydb.Driver) func(context.Context, string) (*repository.User, error) {
	return func(ctx context.Context, username string) (*repository.User, error) {
		var user *repository.User
		err := ydb.Table().Do(ctx, func(ctx context.Context, sess *ydb.Sessions) error {
			res, _, err := sess.Execute(
				user_by_username_query,
				ydb.QueryParams(Int64("$username", username)),
			)
			if err != nil {
				return nil, err
			}
			defer res.Close()
			if err := res.NextResultSet(); err != nil {
				return err
			}
			return user.Scan(res)
		})
		return user, err
	}
}

// HANDLER TO DETECT A USER
func with[A any](
	ctx context.Context,
	userIdent A,
	fn func(context.Context, A) (*repository.User, error),
	byUser func(context.Context, repository.User) (string, error),
) (string, error) {
	user, err := fn(ctx, userIdent)
	if err != nil {
		return err
	}
	return byUser(ctx, *user)
}

func WhoisHandler(
	mux botMux,
	groupChatAdminMiddleware telebot.Middleware,
	userByID func(context.Context, int64) (*repository.User, error),
	userByUsername func(context.Context, string) (*repository.User, error),
	userGroupsByUserId func(context.Context, int64) ([]int64, error),
	log *zap.Logger,
) {
	botMux.Use(groupChatAdminMiddleware)

	byUser := func(ctx context.Context, user repository.User) (string, error) {
		groupIDs, err := userGroupByUserId(ctx, user.ID)
		return fmt.Sprintf(
			"Пользователь %#v\nУчаствует в группах %v",
			user, groupIDs,
		), nil
	}

	whois := func(ctx telebot.Context) error {
		args := ctx.Args()
		if len(args) == 0 {
			return ctx.EditOrReply("Введите имя пользователя или его идентификатор")
		}
		stdctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		userID, err := strings.Atoi(args[0])
		var message string
		var err error
		if err != nil {
			message, err = with(stdctx, args[0], userByUsername, byUser)
		} else {
			message, err = with(stdctx, userID, userByID, byUser)
		}
		if err != nil {
			ctx.EditOrReply("Ошибка получения информации о пользователе")
			log.Error("Ошибка получения информации о пользователе", zap.Error(err))
			return nil
		}
		return ctx.EditOrReply(message)
	}
	botMux.Handle("/whois", whois)
}
