import { configureStore } from "@reduxjs/toolkit";
import menuReducer from "./menu";
import authReducer from "./auth";
import chatReducer from "./chat";
import quotaReducer from "./quota";
import packageReducer from "./package";
import subscriptionReducer from "./subscription";
import apiReducer from "./api";

const store = configureStore({
  reducer: {
    menu: menuReducer,
    auth: authReducer,
    chat: chatReducer,
    quota: quotaReducer,
    package: packageReducer,
    subscription: subscriptionReducer,
    api: apiReducer,
  },
});

type RootState = ReturnType<typeof store.getState>;
type AppDispatch = typeof store.dispatch;

export type { RootState, AppDispatch };
export default store;