package com.shiftcryptosecurity.bitbox;

import android.annotation.SuppressLint;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.content.res.Configuration;
import android.hardware.usb.UsbManager;
import androidx.appcompat.app.AppCompatActivity;
import androidx.lifecycle.ViewModelProviders;

import android.os.Handler;
import android.os.Message;
import android.os.Bundle;
import android.webkit.JavascriptInterface;
import android.webkit.MimeTypeMap;
import android.webkit.WebResourceRequest;
import android.webkit.WebResourceResponse;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.webkit.WebChromeClient;
import android.webkit.ConsoleMessage;
import android.util.Log;
import android.widget.Toast;

import java.io.InputStream;
import java.util.Map;

import goserver.Goserver;

public class MainActivity extends AppCompatActivity {
    private class JavascriptBridge {
        private Context context;

        JavascriptBridge(Context context) {
            this.context = context;
        }

        @JavascriptInterface
        public void call(int queryID, String query) {
            Goserver.backendCall(queryID, query);
        }
    }

    private final BroadcastReceiver broadcastReceiver = new BroadcastReceiver() {
        public void onReceive(Context context, Intent intent) {
            handleIntent(intent);
        }
    };;

    private static String getMimeType(String url) {
        String type = null;
        String extension = MimeTypeMap.getFileExtensionFromUrl(url);
        if (extension != null) {
            switch (extension) {
                case "js":
                    type = "text/javascript";
                    break;
                default:
                    type = MimeTypeMap.getSingleton().getMimeTypeFromExtension(extension);
                    break;
            }
        }

        return type;
    }

    @Override
    public void onConfigurationChanged(Configuration newConfig){
        super.onConfigurationChanged(newConfig);
    }

    @SuppressLint("HandlerLeak")
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        getSupportActionBar().hide(); // hide title bar with app name.
        setContentView(R.layout.activity_main);
        final WebView vw = (WebView)findViewById(R.id.vw);

        // GoModel invokes the Go backend. It is in a ViewModel so it only runs once, not every time
        // onCreate is called (on a configuration change like orientation change)
        final GoViewModel goViewModel = ViewModelProviders.of(this).get(GoViewModel.class);
        goViewModel.setMessageHandlers(
                new Handler() {
                    @Override
                    public void handleMessage(final Message msg) {
                        final GoViewModel.Response response = (GoViewModel.Response) msg.obj;
                        runOnUiThread(new Runnable() {
                            @Override
                            public void run() {
                                vw.evaluateJavascript("window.onAndroidCallResponse(" + String.valueOf(response.queryID) + ", " + response.response + ");", null);
                            }
                        });
                    }
                },
                new Handler() {
                    @Override
                    public void handleMessage(final Message msg) {
                        runOnUiThread(new Runnable() {
                            @Override
                            public void run() {
                                vw.evaluateJavascript("window.onAndroidPushNotification(" + (String)(msg.obj) + ");", null);
                            }
                        });
                    }
                }
        );
        vw.clearCache(true);
        vw.clearHistory();
        vw.getSettings().setJavaScriptEnabled(true);
        vw.getSettings().setAllowUniversalAccessFromFileURLs(true);

        vw.setWebViewClient(new WebViewClient() {
            @Override
            public WebResourceResponse shouldInterceptRequest(final WebView view, WebResourceRequest request) {
                // Intercept file:/// urls and serve it from the Android assets folder.
                try {
                    String url = request.getUrl().toString();
                    InputStream inputStream = getAssets().open(url.replace("file:///", "web/"));
                    String mimeType = getMimeType(url);
                    if (mimeType != null) {
                        return new WebResourceResponse(mimeType, "UTF-8", inputStream);
                    }
                    log("Unknown MimeType: " + url);
                } catch(Exception e) {
                }
                return super.shouldInterceptRequest(view, request);
            }
        });

        // WebView.setWebContentsDebuggingEnabled(true);
        vw.setWebChromeClient(new WebChromeClient() {
            @Override
            public boolean onConsoleMessage(ConsoleMessage consoleMessage) {
                log(consoleMessage.message() + " -- From line "
                        + consoleMessage.lineNumber() + " of "
                        + consoleMessage.sourceId());
                return super.onConsoleMessage(consoleMessage);
            }
        });
        final String javascriptVariableName = "android";
        vw.addJavascriptInterface(new JavascriptBridge(this), javascriptVariableName);
        vw.loadUrl("file:///android_asset/web/index.html");
    }

    private void log(String msg) {
        Log.d("com.shiftcryptosecurity.bitbox", "XYZ: " + msg);
    }

    @Override
    protected void onNewIntent(Intent intent) {
        // This is only called reliably when USB is attached with android:launchMode="singleTop"
        super.onNewIntent(intent);
        setIntent(intent); // make sure onResume will have access to this intent
    }

    @Override
    protected void onResume() {
        super.onResume();

        // This is only called reliably when USB is attached with android:launchMode="singleTop"

        // Usb device list is updated on ATTACHED / DETACHED intents.
        // ATTACHED intent is an activity intent in AndroidManifest.xml so the app is launched when
        // a device is attached. On launch or when it is already running, onIntent() is called
        // followed by onResume(), where the intent is handled.
        // DETACHED intent is a broadcast intent which we register here.
        IntentFilter filter = new IntentFilter();
        filter.addAction(UsbManager.ACTION_USB_DEVICE_DETACHED);
        registerReceiver(this.broadcastReceiver, filter);
        handleIntent(getIntent());
    }

    @Override
    protected void onPause() {
        super.onPause();
        unregisterReceiver(this.broadcastReceiver);
    }

    private void handleIntent(Intent intent) {
        final GoViewModel goViewModel = ViewModelProviders.of(this).get(GoViewModel.class);
        if (intent.getAction().equals(UsbManager.ACTION_USB_DEVICE_ATTACHED)) {
            goViewModel.updateDeviceList();
        }
        if (intent.getAction().equals(UsbManager.ACTION_USB_DEVICE_DETACHED)) {
            goViewModel.updateDeviceList();
        }
    }
}

