package com.leohalb.fyneforeground;

import android.app.Activity;
import android.content.Intent;
import android.database.Cursor;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.provider.OpenableColumns;

public class MainActivity extends Activity {
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        // Foreground Service starten
        Intent serviceIntent = new Intent(this, GoForegroundService.class);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForegroundService(serviceIntent);
        } else {
            startService(serviceIntent);
        }

        // GoNativeActivity starten
        Intent activityIntent = new Intent(this, org.golang.app.GoNativeActivity.class);
        activityIntent.addFlags(Intent.FLAG_ACTIVITY_NO_ANIMATION);
        startActivity(activityIntent);

        finish();
    }
}
