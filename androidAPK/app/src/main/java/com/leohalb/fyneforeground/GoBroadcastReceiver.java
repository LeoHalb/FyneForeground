package com.leohalb.fyneforeground;

import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;

public class GoBroadcastReceiver extends BroadcastReceiver {
    private final long handlerID;

    public GoBroadcastReceiver(long handlerID) {
        this.handlerID = handlerID;
    }

    @Override
    public void onReceive(Context context, Intent intent) {
        GoAbstractDispatch.invoke(handlerID, "onReceive", new Object[]{context, intent});
    }
}