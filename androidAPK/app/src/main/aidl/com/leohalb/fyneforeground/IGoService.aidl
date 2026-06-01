package com.leohalb.fyneforeground;

interface IGoService {
    String getElapsed();
    void start(long timestamp);
    String stop();
    boolean isRunning();
}

